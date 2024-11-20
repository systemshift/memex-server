package memex

import (
	"time"

	"memex/internal/memex/core"
)

// Node represents a node in the memex
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
	Source      string
	Target      string
	Type        string
	Meta        map[string]any
	SourceChunk string
	TargetChunk string
}

// Convert core.Node to Node
func fromCoreNode(node core.Node, content []byte) Node {
	return Node{
		ID:       node.ID,
		Type:     node.Type,
		Meta:     node.Meta,
		Content:  content,
		Created:  node.Created,
		Modified: node.Modified,
	}
}

// Convert core.Link to Link
func fromCoreLink(link core.Link) Link {
	return Link{
		Source:      link.Source,
		Target:      link.Target,
		Type:        link.Type,
		Meta:        link.Meta,
		SourceChunk: link.SourceChunk,
		TargetChunk: link.TargetChunk,
	}
}

// Convert []core.Link to []Link
func fromCoreLinks(links []core.Link) []Link {
	result := make([]Link, len(links))
	for i, link := range links {
		result[i] = fromCoreLink(link)
	}
	return result
}

// Convert []core.Node to []Node
func fromCoreNodes(nodes []core.Node) []Node {
	result := make([]Node, len(nodes))
	for i, node := range nodes {
		result[i] = fromCoreNode(node, nil) // Content not loaded for lists
	}
	return result
}
