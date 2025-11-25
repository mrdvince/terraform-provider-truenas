package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type mockTrueNASServer struct {
	server      *httptest.Server
	connections []*websocket.Conn
	mu          sync.Mutex
	handler     func(conn *websocket.Conn, msg map[string]any)
}

func newMockTrueNASServer(handler func(conn *websocket.Conn, msg map[string]any)) *mockTrueNASServer {
	mock := &mockTrueNASServer{handler: handler}

	mock.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/websocket" {
			http.NotFound(w, r)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		mock.mu.Lock()
		mock.connections = append(mock.connections, conn)
		mock.mu.Unlock()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var msg map[string]any
			if err := json.Unmarshal(message, &msg); err != nil {
				continue
			}

			if mock.handler != nil {
				mock.handler(conn, msg)
			}
		}
	}))

	return mock
}

func (m *mockTrueNASServer) URL() string {
	return strings.Replace(m.server.URL, "http://", "ws://", 1)
}

func (m *mockTrueNASServer) Close() {
	m.mu.Lock()
	for _, conn := range m.connections {
		conn.Close()
	}
	m.mu.Unlock()
	m.server.Close()
}

func defaultHandler(conn *websocket.Conn, msg map[string]any) {
	msgType, _ := msg["msg"].(string)

	switch msgType {
	case "connect":
		conn.WriteJSON(map[string]any{"msg": "connected"})
	case "method":
		id, _ := msg["id"].(string)
		method, _ := msg["method"].(string)

		switch method {
		case "auth.login_with_api_key":
			conn.WriteJSON(map[string]any{
				"id":     id,
				"msg":    "result",
				"result": true,
			})
		default:
			conn.WriteJSON(map[string]any{
				"id":     id,
				"msg":    "result",
				"result": nil,
			})
		}
	}
}

func TestNewClient_Success(t *testing.T) {
	mock := newMockTrueNASServer(defaultHandler)
	defer mock.Close()

	client, err := NewClient(mock.URL(), "test-api-key")

	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_InvalidURL(t *testing.T) {
	client, err := NewClient("ws://localhost:99999", "test-api-key")

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "failed to connect to websocket")
}

func TestNewClient_ConnectTimeout(t *testing.T) {
	mock := newMockTrueNASServer(func(conn *websocket.Conn, msg map[string]any) {
		// never respond to connect message
	})
	defer mock.Close()

	client, err := NewClient(mock.URL(), "test-api-key")

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "timeout waiting for connected message")
}

func TestNewClient_AuthFailure(t *testing.T) {
	mock := newMockTrueNASServer(func(conn *websocket.Conn, msg map[string]any) {
		msgType, _ := msg["msg"].(string)

		switch msgType {
		case "connect":
			conn.WriteJSON(map[string]any{"msg": "connected"})
		case "method":
			id, _ := msg["id"].(string)
			conn.WriteJSON(map[string]any{
				"id":  id,
				"msg": "result",
				"error": map[string]any{
					"error":      "Invalid API key",
					"error_code": 401,
				},
			})
		}
	})
	defer mock.Close()

	client, err := NewClient(mock.URL(), "bad-api-key")

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "api error")
}

func TestClient_Call_Success(t *testing.T) {
	expectedResult := map[string]string{"version": "24.04.0"}

	mock := newMockTrueNASServer(func(conn *websocket.Conn, msg map[string]any) {
		msgType, _ := msg["msg"].(string)

		switch msgType {
		case "connect":
			conn.WriteJSON(map[string]any{"msg": "connected"})
		case "method":
			id, _ := msg["id"].(string)
			method, _ := msg["method"].(string)

			if method == "auth.login_with_api_key" {
				conn.WriteJSON(map[string]any{"id": id, "msg": "result", "result": true})
				return
			}

			if method == "system.info" {
				conn.WriteJSON(map[string]any{"id": id, "msg": "result", "result": expectedResult})
				return
			}
		}
	})
	defer mock.Close()

	client, err := NewClient(mock.URL(), "test-api-key")
	require.NoError(t, err)

	resp, err := client.Call(context.Background(), "system.info", nil)

	require.NoError(t, err)
	assert.NotNil(t, resp)

	var result map[string]string
	err = json.Unmarshal(resp.Result, &result)
	require.NoError(t, err)
	assert.Equal(t, "24.04.0", result["version"])
}

func TestClient_Call_APIError(t *testing.T) {
	mock := newMockTrueNASServer(func(conn *websocket.Conn, msg map[string]any) {
		msgType, _ := msg["msg"].(string)

		switch msgType {
		case "connect":
			conn.WriteJSON(map[string]any{"msg": "connected"})
		case "method":
			id, _ := msg["id"].(string)
			method, _ := msg["method"].(string)

			if method == "auth.login_with_api_key" {
				conn.WriteJSON(map[string]any{"id": id, "msg": "result", "result": true})
				return
			}

			conn.WriteJSON(map[string]any{
				"id":  id,
				"msg": "result",
				"error": map[string]any{
					"error":      "Pool not found",
					"error_code": 404,
				},
			})
		}
	})
	defer mock.Close()

	client, err := NewClient(mock.URL(), "test-api-key")
	require.NoError(t, err)

	resp, err := client.Call(context.Background(), "pool.query", nil)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "Pool not found")
	assert.Contains(t, err.Error(), "404")
}

func TestClient_Call_ContextCancellation(t *testing.T) {
	mock := newMockTrueNASServer(func(conn *websocket.Conn, msg map[string]any) {
		msgType, _ := msg["msg"].(string)

		switch msgType {
		case "connect":
			conn.WriteJSON(map[string]any{"msg": "connected"})
		case "method":
			id, _ := msg["id"].(string)
			method, _ := msg["method"].(string)

			if method == "auth.login_with_api_key" {
				conn.WriteJSON(map[string]any{"id": id, "msg": "result", "result": true})
				return
			}

			// never respond to simulate slow operation
		}
	})
	defer mock.Close()

	client, err := NewClient(mock.URL(), "test-api-key")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	resp, err := client.Call(ctx, "slow.operation", nil)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestClient_Call_ConcurrentRequests(t *testing.T) {
	mock := newMockTrueNASServer(func(conn *websocket.Conn, msg map[string]any) {
		msgType, _ := msg["msg"].(string)

		switch msgType {
		case "connect":
			conn.WriteJSON(map[string]any{"msg": "connected"})
		case "method":
			id, _ := msg["id"].(string)
			method, _ := msg["method"].(string)

			if method == "auth.login_with_api_key" {
				conn.WriteJSON(map[string]any{"id": id, "msg": "result", "result": true})
				return
			}

			time.Sleep(50 * time.Millisecond)
			conn.WriteJSON(map[string]any{
				"id":     id,
				"msg":    "result",
				"result": map[string]any{"method": method, "id": id},
			})
		}
	})
	defer mock.Close()

	client, err := NewClient(mock.URL(), "test-api-key")
	require.NoError(t, err)

	var wg sync.WaitGroup
	results := make(chan string, 5)
	errors := make(chan error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			method := "test.method"
			resp, err := client.Call(context.Background(), method, []any{idx})
			if err != nil {
				errors <- err
				return
			}

			var result map[string]any
			json.Unmarshal(resp.Result, &result)
			results <- result["id"].(string)
		}(i)
	}

	wg.Wait()
	close(results)
	close(errors)

	for err := range errors {
		t.Errorf("unexpected error: %v", err)
	}

	ids := make(map[string]bool)
	for id := range results {
		ids[id] = true
	}

	assert.Len(t, ids, 5, "all 5 concurrent requests should receive unique responses")
}

func TestClient_WaitForJob_Success(t *testing.T) {
	callCount := 0

	mock := newMockTrueNASServer(func(conn *websocket.Conn, msg map[string]any) {
		msgType, _ := msg["msg"].(string)

		switch msgType {
		case "connect":
			conn.WriteJSON(map[string]any{"msg": "connected"})
		case "method":
			id, _ := msg["id"].(string)
			method, _ := msg["method"].(string)

			if method == "auth.login_with_api_key" {
				conn.WriteJSON(map[string]any{"id": id, "msg": "result", "result": true})
				return
			}

			if method == "core.get_jobs" {
				callCount++
				state := "RUNNING"
				if callCount >= 3 {
					state = "SUCCESS"
				}
				conn.WriteJSON(map[string]any{
					"id":  id,
					"msg": "result",
					"result": []map[string]any{
						{"id": 123, "state": state},
					},
				})
			}
		}
	})
	defer mock.Close()

	client, err := NewClient(mock.URL(), "test-api-key")
	require.NoError(t, err)

	err = client.WaitForJob(context.Background(), 123)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, callCount, 3)
}

func TestClient_WaitForJob_Failure(t *testing.T) {
	mock := newMockTrueNASServer(func(conn *websocket.Conn, msg map[string]any) {
		msgType, _ := msg["msg"].(string)

		switch msgType {
		case "connect":
			conn.WriteJSON(map[string]any{"msg": "connected"})
		case "method":
			id, _ := msg["id"].(string)
			method, _ := msg["method"].(string)

			if method == "auth.login_with_api_key" {
				conn.WriteJSON(map[string]any{"id": id, "msg": "result", "result": true})
				return
			}

			if method == "core.get_jobs" {
				conn.WriteJSON(map[string]any{
					"id":  id,
					"msg": "result",
					"result": []map[string]any{
						{"id": 123, "state": "FAILED", "error": "Disk not found"},
					},
				})
			}
		}
	})
	defer mock.Close()

	client, err := NewClient(mock.URL(), "test-api-key")
	require.NoError(t, err)

	err = client.WaitForJob(context.Background(), 123)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job failed")
}

func TestClient_WaitForJob_NotFound(t *testing.T) {
	mock := newMockTrueNASServer(func(conn *websocket.Conn, msg map[string]any) {
		msgType, _ := msg["msg"].(string)

		switch msgType {
		case "connect":
			conn.WriteJSON(map[string]any{"msg": "connected"})
		case "method":
			id, _ := msg["id"].(string)
			method, _ := msg["method"].(string)

			if method == "auth.login_with_api_key" {
				conn.WriteJSON(map[string]any{"id": id, "msg": "result", "result": true})
				return
			}

			if method == "core.get_jobs" {
				conn.WriteJSON(map[string]any{
					"id":     id,
					"msg":    "result",
					"result": []map[string]any{},
				})
			}
		}
	})
	defer mock.Close()

	client, err := NewClient(mock.URL(), "test-api-key")
	require.NoError(t, err)

	err = client.WaitForJob(context.Background(), 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job 999 not found")
}

func TestClient_WaitForJob_ContextCancellation(t *testing.T) {
	mock := newMockTrueNASServer(func(conn *websocket.Conn, msg map[string]any) {
		msgType, _ := msg["msg"].(string)

		switch msgType {
		case "connect":
			conn.WriteJSON(map[string]any{"msg": "connected"})
		case "method":
			id, _ := msg["id"].(string)
			method, _ := msg["method"].(string)

			if method == "auth.login_with_api_key" {
				conn.WriteJSON(map[string]any{"id": id, "msg": "result", "result": true})
				return
			}

			if method == "core.get_jobs" {
				conn.WriteJSON(map[string]any{
					"id":  id,
					"msg": "result",
					"result": []map[string]any{
						{"id": 123, "state": "RUNNING"},
					},
				})
			}
		}
	})
	defer mock.Close()

	client, err := NewClient(mock.URL(), "test-api-key")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err = client.WaitForJob(ctx, 123)

	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestClient_URLConversion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "https to wss",
			input:    "https://truenas.local",
			expected: "wss://truenas.local/websocket",
		},
		{
			name:     "http to ws",
			input:    "http://truenas.local",
			expected: "ws://truenas.local/websocket",
		},
		{
			name:     "already ws",
			input:    "ws://truenas.local",
			expected: "ws://truenas.local/websocket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := tt.input
			if len(url) > 5 && url[:5] == "https" {
				url = "wss" + url[5:] + "/websocket"
			} else if len(url) > 4 && url[:4] == "http" {
				url = "ws" + url[4:] + "/websocket"
			} else {
				url = url + "/websocket"
			}
			assert.Equal(t, tt.expected, url)
		})
	}
}

func TestNewID_Uniqueness(t *testing.T) {
	c := &Client{
		pending: make(map[string]chan *Response),
	}

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := c.newID()
		assert.False(t, ids[id], "ID should be unique: %s", id)
		ids[id] = true
	}
}

func TestNewID_ThreadSafety(t *testing.T) {
	c := &Client{
		pending: make(map[string]chan *Response),
	}

	var wg sync.WaitGroup
	ids := make(chan string, 100)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				ids <- c.newID()
			}
		}()
	}

	wg.Wait()
	close(ids)

	seen := make(map[string]bool)
	for id := range ids {
		assert.False(t, seen[id], "ID collision detected: %s", id)
		seen[id] = true
	}
}
