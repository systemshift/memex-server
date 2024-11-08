package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	fmt.Println("Welcome to memex editor")
	fmt.Println("Enter your text (type ':wq' on a new line to save and quit)")

	scanner := bufio.NewScanner(os.Stdin)
	var lines []string

	for scanner.Scan() {
		line := scanner.Text()
		if line == ":wq" {
			break
		}
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "reading standard input: %v\n", err)
		os.Exit(1)
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

	content := strings.Join(lines, "\n")
	if _, err := file.WriteString(content); err != nil {
		fmt.Fprintf(os.Stderr, "writing to file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Saved to %s\n", filename)
}
