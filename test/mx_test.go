package test

import (
	"strings"
	"testing"
	"time"

	"memex/internal/memex/mx"
)

func TestMXFormat(t *testing.T) {
	t.Run("Create New Document", func(t *testing.T) {
		content := "This is test content"
		doc := mx.New(content)

		if doc.Version != 1 {
			t.Errorf("Expected version 1, got %d", doc.Version)
		}

		if doc.Content != content {
			t.Errorf("Expected content %q, got %q", content, doc.Content)
		}

		if time.Since(doc.Created) > time.Minute {
			t.Error("Created time is too old")
		}
	})

	t.Run("Document String Format", func(t *testing.T) {
		doc := mx.New("Test content")
		doc.SetTitle("Test Note")
		doc.AddTag("test")
		doc.AddTag("example")

		output := doc.String()

		// Check required parts
		required := []string{
			"---",
			"version: 1",
			"created:",
			"type: note",
			"tags: [test, example]",
			"title: Test Note",
			"Test content",
		}

		for _, req := range required {
			if !strings.Contains(output, req) {
				t.Errorf("Output missing %q", req)
			}
		}
	})

	t.Run("Parse Document", func(t *testing.T) {
		input := `---
version: 1
created: 2024-11-11T08:45:23Z
type: note
tags: [test, example]
title: Test Note
---
Test content
More content`

		doc, err := mx.Parse(input)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}

		if doc.Title != "Test Note" {
			t.Errorf("Expected title %q, got %q", "Test Note", doc.Title)
		}

		if !strings.Contains(doc.Content, "Test content") {
			t.Error("Content not parsed correctly")
		}
	})

	t.Run("Tag Management", func(t *testing.T) {
		doc := mx.New("content")

		// Add tags
		doc.AddTag("test")
		doc.AddTag("example")

		if len(doc.Tags) != 2 {
			t.Errorf("Expected 2 tags, got %d", len(doc.Tags))
		}

		// Add duplicate tag
		doc.AddTag("test")
		if len(doc.Tags) != 2 {
			t.Error("Duplicate tag was added")
		}

		// Remove tag
		doc.RemoveTag("test")
		if len(doc.Tags) != 1 {
			t.Error("Tag not removed")
		}
		if doc.Tags[0] != "example" {
			t.Error("Wrong tag removed")
		}
	})
}
