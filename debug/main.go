package main

import (
	"context"
	"encoding/json"
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

	c, err := client.NewClient(host, apiKey)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()

	fmt.Println("Querying unused disks...")
	resp, err := c.Call(ctx, "disk.get_unused", nil)
	if err != nil {
		log.Fatalf("disk.get_unused failed: %v", err)
	}

	var disks []map[string]any
	if err := json.Unmarshal(resp.Result, &disks); err != nil {
		log.Fatalf("failed to parse disks: %v", err)
	}

	fmt.Printf("Found %d unused disks.\n", len(disks))
	for _, d := range disks {
		fmt.Printf("- %s (size: %v, type: %v)\n", d["name"], d["size"], d["type"])
	}
}
