package memex

import (
	"time"
)

// Node represents a node in the graph
type Node struct {
	ID       string
	Type     string
	Meta     map[string]any
	Content  []byte
	Created  time.Time
	Modified time.Time
}

// Link represents a relationship between nodes
type Link struct {
	Target string
	Type   string
	Meta   map[string]any
}
