package memex

import "time"

// Commit represents a snapshot of content in the repository
type Commit struct {
	Hash      string    // Unique identifier for the commit
	Message   string    // Commit message
	Timestamp time.Time // When the commit was created
}

// File represents a file in the repository
type File struct {
	Name     string    // File name
	Path     string    // Full path to file
	Modified time.Time // Last modification time
}

// Status represents the current state of the repository
type Status struct {
	LastCommit       *Commit // Most recent commit, if any
	UncommittedFiles []File  // Files not yet committed
}

// Config represents repository configuration
type Config struct {
	RootDirectory string            // Root directory of the repository
	Settings      map[string]string // Additional settings
}
