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
	apiKey := os.Getenv("TRUENAS_DEV_KEY")

	if host == "" || apiKey == "" {
		log.Fatal("TRUENAS_HOST and TRUENAS_DEV_KEY must be set")
	}

	c, err := client.NewClient(host, apiKey)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	fmt.Println("=== TrueNAS App API Research ===")

	fmt.Println("\n--- app.query (installed apps) ---")
	resp, err := c.Call(ctx, "app.query", nil)
	if err != nil {
		log.Printf("app.query error: %v", err)
	} else {
		prettyPrint("installed apps", resp.Result)
	}

	fmt.Println("\n--- core.get_methods (app methods) ---")
	resp, err = c.Call(ctx, "core.get_methods", nil)
	if err != nil {
		log.Printf("core.get_methods error: %v", err)
	} else {
		var methods map[string]any
		json.Unmarshal(resp.Result, &methods)
		for name := range methods {
			if len(name) > 4 && name[:4] == "app." {
				fmt.Println(name)
			}
		}
	}

	fmt.Println("\n--- immich from app.available ---")
	resp, err = c.Call(ctx, "app.available", []any{
		[][]any{{"name", "=", "immich"}},
	})
	if err != nil {
		log.Printf("app.available error: %v", err)
	} else {
		prettyPrint("immich info", resp.Result)
	}

	fmt.Println("\n--- check syncthing ---")
	resp, err = c.Call(ctx, "app.available", []any{
		[][]any{{"name", "=", "syncthing"}},
	})
	if err != nil {
		log.Printf("syncthing check error: %v", err)
	} else {
		prettyPrint("syncthing", resp.Result)
	}
}

func prettyPrint(label string, data json.RawMessage) {
	var parsed any
	json.Unmarshal(data, &parsed)
	pretty, _ := json.MarshalIndent(parsed, "", "  ")
	fmt.Printf("%s:\n%s\n", label, string(pretty))
}
