package memex

import "time"

// Node represents a node in the repository
type Node struct {
	ID       string
	Type     string
	Content  []byte
	Meta     map[string]interface{}
	Created  time.Time
	Modified time.Time
}

// Link represents a link between nodes
type Link struct {
	Source   string
	Target   string
	Type     string
	Meta     map[string]interface{}
	Created  time.Time
	Modified time.Time
}
