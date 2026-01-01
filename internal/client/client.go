package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	clientCache   = make(map[string]*Client)
	clientCacheMu sync.Mutex
)

type Client struct {
	conn      *websocket.Conn
	host      string
	apiKey    string
	mu        sync.Mutex
	writeMu   sync.Mutex
	pending   map[string]chan *Response
	nextID    int
	connected chan struct{}
	closed    bool
}

type Request struct {
	ID     string `json:"id"`
	Msg    string `json:"msg"`
	Method string `json:"method,omitempty"`
	Params []any  `json:"params,omitempty"`
}

type Response struct {
	ID     string          `json:"id"`
	Msg    string          `json:"msg"`
	Error  *ResponseError  `json:"error,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

type ResponseError struct {
	Error            int    `json:"error"`
	ErrName          string `json:"errname"`
	Reason           string `json:"reason"`
	Trace            any    `json:"trace,omitempty"`
	Extra            any    `json:"extra,omitempty"`
}

func NewClient(host, apiKey string) (*Client, error) {
	cacheKey := host + ":" + apiKey

	clientCacheMu.Lock()
	if cached, ok := clientCache[cacheKey]; ok && !cached.closed {
		clientCacheMu.Unlock()
		return cached, nil
	}
	clientCacheMu.Unlock()

	// truenas websocket url
	url := fmt.Sprintf("%s/websocket", host)
	// handle wss if https is provided
	if len(host) > 5 && host[:5] == "https" {
		url = fmt.Sprintf("wss%s/websocket", host[5:])
	} else if len(host) > 4 && host[:4] == "http" {
		url = fmt.Sprintf("ws%s/websocket", host[4:])
	}

	// TODO: make TLS verification configurable
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		Proxy:           http.ProxyFromEnvironment,
	}

	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to websocket: %w", err)
	}

	c := &Client{
		conn:      conn,
		host:      host,
		apiKey:    apiKey,
		pending:   make(map[string]chan *Response),
		connected: make(chan struct{}),
	}

	go c.readLoop()

	if err := c.connect(); err != nil {
		return nil, err
	}

	if err := c.auth(); err != nil {
		return nil, err
	}

	clientCacheMu.Lock()
	clientCache[cacheKey] = c
	clientCacheMu.Unlock()

	return c, nil
}

func (c *Client) readLoop() {
	defer c.conn.Close()
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("Read error: %v", err)
			return
		}
		log.Printf("DEBUG: Received message: %s", string(message))

		var resp Response
		if err := json.Unmarshal(message, &resp); err != nil {
			log.Printf("Unmarshal error: %v", err)
			continue
		}

		switch resp.Msg {
		case "result", "method":
			c.mu.Lock()
			ch, ok := c.pending[resp.ID]
			c.mu.Unlock()
			if ok {
				ch <- &resp
			}
		case "connected":
			close(c.connected)
		}
	}
}

func (c *Client) connect() error {
	// truenas websocket protocol requires a connect handshake first
	// { "msg": "connect", "version": "1", "support": ["1"] }
	req := map[string]any{
		"msg":     "connect",
		"version": "1",
		"support": []string{"1"},
	}

	if err := c.conn.WriteJSON(req); err != nil {
		return err
	}

	// wait for the "connected" message from the server
	select {
	case <-c.connected:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for connected message")
	}
}

func (c *Client) auth() error {
	_, err := c.Call(context.Background(), "auth.login_with_api_key", []any{c.apiKey})
	return err
}

func (c *Client) Call(ctx context.Context, method string, params []any) (*Response, error) {
	id := c.newID()
	ch := make(chan *Response, 1)

	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	req := Request{
		ID:     id,
		Msg:    "method",
		Method: method,
		Params: params,
	}

	c.writeMu.Lock()
	err := c.conn.WriteJSON(req)
	c.writeMu.Unlock()
	if err != nil {
		return nil, err
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("api error [%s]: %s", resp.Error.ErrName, resp.Error.Reason)
		}
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(120 * time.Second):
		return nil, fmt.Errorf("timeout waiting for response to %s", method)
	}
}

func (c *Client) WaitForJob(ctx context.Context, jobID int64) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			resp, err := c.Call(ctx, "core.get_jobs", []any{[]any{[]any{"id", "=", jobID}}})
			if err != nil {
				return fmt.Errorf("failed to poll job %d: %w", jobID, err)
			}

			var jobs []map[string]any
			if err := json.Unmarshal(resp.Result, &jobs); err != nil {
				return fmt.Errorf("failed to parse job status: %w", err)
			}

			if len(jobs) == 0 {
				return fmt.Errorf("job %d not found", jobID)
			}

			job := jobs[0]
			state := job["state"].(string)

			switch state {
			case "SUCCESS":
				return nil
			case "FAILED":
				return fmt.Errorf("job failed: %v", job["error"])
			}
		}
	}
}

func (c *Client) newID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	return fmt.Sprintf("%d", c.nextID)
}

func (c *Client) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()

	clientCacheMu.Lock()
	for key, cached := range clientCache {
		if cached == c {
			delete(clientCache, key)
			break
		}
	}
	clientCacheMu.Unlock()

	return c.conn.Close()
}

// ResetClientCache clears the client cache, useful for tests
func ResetClientCache() {
	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()
	for key, c := range clientCache {
		c.conn.Close()
		delete(clientCache, key)
	}
}
