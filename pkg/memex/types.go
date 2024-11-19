package memex

import (
	"time"

	"memex/internal/memex/core"
)

// Object represents a stored object in the memex
type Object struct {
	ID       string
	Content  []byte
	Type     string
	Created  time.Time
	Modified time.Time
	Meta     map[string]any
}

// Link represents a relationship between objects
type Link struct {
	Source      string
	Target      string
	Type        string
	Meta        map[string]any
	SourceChunk string
	TargetChunk string
}

// Convert internal core.Node to public Object
func convertNode(node core.Node) Object {
	return Object{
		ID:       node.ID,
		Type:     node.Type,
		Created:  node.Created,
		Modified: node.Modified,
		Meta:     node.Meta,
	}
}

// Convert internal core.Link to public Link
func convertLink(link core.Link) Link {
	return Link{
		Source:      link.Source,
		Target:      link.Target,
		Type:        link.Type,
		Meta:        link.Meta,
		SourceChunk: link.SourceChunk,
		TargetChunk: link.TargetChunk,
	}
}
