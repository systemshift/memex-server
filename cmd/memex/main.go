package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const defaultServerURL = "http://localhost:8080"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	serverURL := getEnv("MEMEX_URL", defaultServerURL)
	command := os.Args[1]

	switch command {
	case "ingest":
		if len(os.Args) < 3 {
			fmt.Println("Usage: memex ingest <content> [format]")
			os.Exit(1)
		}
		format := ""
		if len(os.Args) >= 4 {
			format = os.Args[3]
		}
		ingest(serverURL, os.Args[2], format)

	case "create-node":
		if len(os.Args) < 4 {
			fmt.Println("Usage: memex create-node <id> <type>")
			os.Exit(1)
		}
		createNode(serverURL, os.Args[2], os.Args[3])

	case "get-node":
		if len(os.Args) < 3 {
			fmt.Println("Usage: memex get-node <id>")
			os.Exit(1)
		}
		getNode(serverURL, os.Args[2])

	case "list-nodes":
		listNodes(serverURL)

	case "create-link":
		if len(os.Args) < 5 {
			fmt.Println("Usage: memex create-link <source> <target> <type>")
			os.Exit(1)
		}
		createLink(serverURL, os.Args[2], os.Args[3], os.Args[4])

	case "get-links":
		if len(os.Args) < 3 {
			fmt.Println("Usage: memex get-links <node-id>")
			os.Exit(1)
		}
		getLinks(serverURL, os.Args[2])

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func ingest(serverURL, content, format string) {
	data := map[string]interface{}{
		"content": content,
	}
	if format != "" {
		data["format"] = format
	}

	resp, err := postJSON(serverURL+"/api/ingest", data)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Content ingested: %s\n", resp)
}

func createNode(serverURL, id, nodeType string) {
	data := map[string]interface{}{
		"id":   id,
		"type": nodeType,
		"meta": map[string]interface{}{},
	}

	resp, err := postJSON(serverURL+"/api/nodes", data)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Node created: %s\n", resp)
}

func getNode(serverURL, id string) {
	resp, err := get(serverURL + "/api/nodes/" + id)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(resp)
}

func listNodes(serverURL string) {
	resp, err := get(serverURL + "/api/nodes")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(resp)
}

func createLink(serverURL, source, target, linkType string) {
	data := map[string]interface{}{
		"source": source,
		"target": target,
		"type":   linkType,
		"meta":   map[string]interface{}{},
	}

	resp, err := postJSON(serverURL+"/api/links", data)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Link created: %s\n", resp)
}

func getLinks(serverURL, nodeID string) {
	resp, err := get(serverURL + "/api/nodes/" + nodeID + "/links")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(resp)
}

func postJSON(url string, data interface{}) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("server error: %s", string(body))
	}

	return string(body), nil
}

func get(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("server error: %s", string(body))
	}

	return string(body), nil
}

func printUsage() {
	fmt.Println("Memex CLI - Layered knowledge graphs")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  memex ingest <content> [format]         Ingest raw content")
	fmt.Println("  memex create-node <id> <type>           Create a new node")
	fmt.Println("  memex get-node <id>                     Get node by ID")
	fmt.Println("  memex list-nodes                        List all nodes")
	fmt.Println("  memex create-link <source> <target> <type>  Create a link")
	fmt.Println("  memex get-links <node-id>               Get links for a node")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  MEMEX_URL                               Server URL (default: http://localhost:8080)")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
