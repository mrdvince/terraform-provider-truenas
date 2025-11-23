package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"truenas/internal/client"
)

func main() {
	host := os.Getenv("TRUENAS_HOST")
	if host == "" {
		host = os.Getenv("truenas_host")
	}
	if host == "" {
		log.Fatal("TRUENAS_HOST or truenas_host must be set")
	}
	apiKey := os.Getenv("TRUENAS_DEV_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("truenas_dev_key")
	}

	if apiKey == "" {
		log.Fatal("TRUENAS_DEV_KEY or truenas_dev_key must be set")
	}

	fmt.Printf("Connecting to %s with key %s...\n", host, apiKey[:5]+"...")

	c, err := client.NewClient(host, apiKey)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	fmt.Println("Client created and authenticated!")

	// try to get system info
	resp, err := c.Call(context.Background(), "system.info", nil)
	if err != nil {
		log.Fatalf("Failed to call system.info: %v", err)
	}

	fmt.Printf("System Info: %s\n", string(resp.Result))
}
