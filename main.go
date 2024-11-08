package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	fmt.Println("Welcome to memex editor")
	fmt.Println("Press any key to start editing (Ctrl-S to save, Ctrl-Q to quit)")

	// Create a new editor instance
	editor := NewEditor()

	// Run the editor and get content
	content, err := editor.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "editor error: %v\n", err)
		os.Exit(1)
	}

	// If no content or user quit, exit
	if content == "" {
		fmt.Println("\nNo content saved")
		return
	}

	// Create notes directory if it doesn't exist
	if err := os.MkdirAll("notes", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "creating notes directory: %v\n", err)
		os.Exit(1)
	}

	// Use shorter timestamp (last 5 digits) plus hour/minute for uniqueness
	timestamp := time.Now().Unix() % 100000
	timeStr := time.Now().Format("1504") // HHMM format
	filename := fmt.Sprintf("notes/%d_%s", timestamp, timeStr)

	file, err := os.Create(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		fmt.Fprintf(os.Stderr, "writing to file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nSaved to %s\n", filename)
}
