package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Commit represents a snapshot of content
type Commit struct {
	Hash      string    `json:"hash"`      // Hash of the content
	Message   string    `json:"message"`   // Commit message
	Timestamp time.Time `json:"timestamp"` // When the commit was made
}

// Repository manages version control operations
type Repository struct {
	rootDir string // Root directory for the repository
}

// NewRepository creates a new repository instance
func NewRepository(rootDir string) *Repository {
	return &Repository{
		rootDir: rootDir,
	}
}

// Initialize sets up the repository structure
func (r *Repository) Initialize() error {
	dirs := []string{
		filepath.Join(r.rootDir, ".memex", "objects"),
		filepath.Join(r.rootDir, ".memex", "commits"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}
	return nil
}

// hashContent generates a SHA-256 hash of content
func hashContent(content []byte) string {
	hash := sha256.New()
	hash.Write(content)
	return hex.EncodeToString(hash.Sum(nil))
}

// StoreObject stores content in the objects directory
func (r *Repository) StoreObject(content []byte) (string, error) {
	hash := hashContent(content)

	// Create object path: first 2 chars as directory, rest as filename
	objDir := filepath.Join(r.rootDir, ".memex", "objects", hash[:2])
	objPath := filepath.Join(objDir, hash[2:])

	// Create directory if it doesn't exist
	if err := os.MkdirAll(objDir, 0755); err != nil {
		return "", fmt.Errorf("creating object directory: %w", err)
	}

	// Write content only if it doesn't exist
	if _, err := os.Stat(objPath); os.IsNotExist(err) {
		if err := os.WriteFile(objPath, content, 0644); err != nil {
			return "", fmt.Errorf("writing object file: %w", err)
		}
	}

	return hash, nil
}

// CreateCommit creates a new commit with the given content and message
func (r *Repository) CreateCommit(content []byte, message string) error {
	// Store the content
	hash, err := r.StoreObject(content)
	if err != nil {
		return fmt.Errorf("storing object: %w", err)
	}

	// Create commit object
	commit := Commit{
		Hash:      hash,
		Message:   message,
		Timestamp: time.Now(),
	}

	// Convert commit to JSON
	commitData, err := json.MarshalIndent(commit, "", "    ")
	if err != nil {
		return fmt.Errorf("marshaling commit: %w", err)
	}

	// Save commit
	commitPath := filepath.Join(r.rootDir, ".memex", "commits", fmt.Sprintf("%d.json", commit.Timestamp.Unix()))
	if err := os.WriteFile(commitPath, commitData, 0644); err != nil {
		return fmt.Errorf("writing commit file: %w", err)
	}

	return nil
}

// GetCommits returns all commits in chronological order
func (r *Repository) GetCommits() ([]Commit, error) {
	commitsDir := filepath.Join(r.rootDir, ".memex", "commits")
	entries, err := os.ReadDir(commitsDir)
	if err != nil {
		return nil, fmt.Errorf("reading commits directory: %w", err)
	}

	var commits []Commit
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(commitsDir, entry.Name()))
			if err != nil {
				return nil, fmt.Errorf("reading commit file: %w", err)
			}

			var commit Commit
			if err := json.Unmarshal(data, &commit); err != nil {
				return nil, fmt.Errorf("unmarshaling commit: %w", err)
			}

			commits = append(commits, commit)
		}
	}

	return commits, nil
}

// RestoreCommit restores content from a specific commit
func (r *Repository) RestoreCommit(hash string) ([]byte, error) {
	// Find object file
	objPath := filepath.Join(r.rootDir, ".memex", "objects", hash[:2], hash[2:])

	content, err := os.ReadFile(objPath)
	if err != nil {
		return nil, fmt.Errorf("reading object file: %w", err)
	}

	return content, nil
}
