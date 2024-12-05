package migration

import "time"

// Version is the current export format version
const Version = 1

// Conflict resolution strategies
const (
	Skip    = "skip"    // Skip importing conflicting nodes
	Replace = "replace" // Replace existing nodes with imported ones
	Rename  = "rename"  // Rename imported nodes to avoid conflicts
)

// ExportManifest contains metadata about an export
type ExportManifest struct {
	Version  int       `json:"version"`
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
	Nodes    int       `json:"nodes"`
	Edges    int       `json:"edges"`
	Chunks   int       `json:"chunks"`
}

// ImportOptions configures import behavior
type ImportOptions struct {
	Prefix     string // Prefix to add to imported node IDs
	OnConflict string // How to handle ID conflicts (skip/replace/rename)
	Merge      bool   // Whether to merge with existing content
}
