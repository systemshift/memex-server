package main

// memexd is the web server implementation of memex
// It provides a web interface to interact with memex repositories
// and exposes a REST API for programmatic access

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	fmt.Println("Memex Web Server - Coming Soon")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
