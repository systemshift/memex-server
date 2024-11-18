package memex

import (
	"time"

	"memex/internal/memex/core"
)

// Object represents stored content with metadata
type Object struct {
	ID       string         // Unique identifier
	Content  []byte         // Raw content
	Chunks   []string       // List of chunk hashes that make up the content
	Type     string         // Content type
	Version  int            // Version number
	Created  time.Time      // Creation timestamp
	Modified time.Time      // Last modified
	Meta     map[string]any // Flexible metadata
}

// Link represents relationship between objects
type Link struct {
	Source      string         // Source object ID
	Target      string         // Target object ID
	Type        string         // Relationship type
	Meta        map[string]any // Link metadata
	SourceChunk string         // Optional: specific chunk in source object
	TargetChunk string         // Optional: specific chunk in target object
}

// convertObject converts a core.Object to a memex.Object
func convertObject(obj core.Object) Object {
	return Object{
		ID:       obj.ID,
		Content:  obj.Content,
		Chunks:   obj.Chunks,
		Type:     obj.Type,
		Version:  obj.Version,
		Created:  obj.Created,
		Modified: obj.Modified,
		Meta:     obj.Meta,
	}
}

// convertLink converts a core.Link to a memex.Link
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
