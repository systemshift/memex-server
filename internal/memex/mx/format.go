package mx

import (
	"fmt"
	"strings"
	"time"
)

// Document represents a .mx file
type Document struct {
	// Metadata
	Version int       `json:"version"`
	Created time.Time `json:"created"`
	Type    string    `json:"type"`
	Tags    []string  `json:"tags"`
	Title   string    `json:"title"`

	// Content
	Content string `json:"content"`
}

// Parse parses a .mx file content into a Document
func Parse(content string) (*Document, error) {
	parts := strings.Split(content, "---\n")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid mx format: missing metadata separators")
	}

	// Initialize document with defaults
	doc := &Document{
		Version: 1,
		Created: time.Now(),
		Type:    "note",
		Tags:    []string{},
	}

	// Parse metadata
	metadataLines := strings.Split(strings.TrimSpace(parts[1]), "\n")
	for _, line := range metadataLines {
		key, value, found := strings.Cut(line, ": ")
		if !found {
			continue
		}

		switch key {
		case "version":
			fmt.Sscanf(value, "%d", &doc.Version)
		case "created":
			doc.Created, _ = time.Parse(time.RFC3339, value)
		case "type":
			doc.Type = value
		case "tags":
			// Remove brackets and split
			tags := strings.Trim(value, "[]")
			if tags != "" {
				for _, tag := range strings.Split(tags, ", ") {
					doc.Tags = append(doc.Tags, strings.TrimSpace(tag))
				}
			}
		case "title":
			doc.Title = value
		}
	}

	// Join remaining parts as content (in case content contains ---)
	doc.Content = strings.Join(parts[2:], "---\n")

	return doc, nil
}

// String converts a Document back to .mx format
func (d *Document) String() string {
	var b strings.Builder

	// Write metadata header
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("version: %d\n", d.Version))
	b.WriteString(fmt.Sprintf("created: %s\n", d.Created.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("type: %s\n", d.Type))
	if len(d.Tags) > 0 {
		b.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(d.Tags, ", ")))
	}
	if d.Title != "" {
		b.WriteString(fmt.Sprintf("title: %s\n", d.Title))
	}
	b.WriteString("---\n")

	// Write content
	b.WriteString(d.Content)

	return b.String()
}

// New creates a new Document with default metadata
func New(content string) *Document {
	return &Document{
		Version: 1,
		Created: time.Now(),
		Type:    "note",
		Tags:    []string{},
		Content: content,
	}
}

// SetTitle sets the document title
func (d *Document) SetTitle(title string) {
	d.Title = title
}

// AddTag adds a tag to the document
func (d *Document) AddTag(tag string) {
	// Check if tag already exists
	for _, t := range d.Tags {
		if t == tag {
			return
		}
	}
	d.Tags = append(d.Tags, tag)
}

// RemoveTag removes a tag from the document
func (d *Document) RemoveTag(tag string) {
	for i, t := range d.Tags {
		if t == tag {
			d.Tags = append(d.Tags[:i], d.Tags[i+1:]...)
			return
		}
	}
}

// GetFilesFromCommit extracts filenames from commit content
func GetFilesFromCommit(content string) map[string]struct{} {
	files := make(map[string]struct{})
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "--- ") && strings.HasSuffix(line, " ---") {
			filename := strings.TrimPrefix(line, "--- ")
			filename = strings.TrimSuffix(filename, " ---")
			files[filename] = struct{}{}
		}
	}
	return files
}
