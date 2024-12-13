package types

// ModuleCapability represents a specific capability a module provides
type ModuleCapability string

// Module defines the interface that all memex modules must implement
type Module interface {
	// ID returns the unique identifier for this module
	ID() string

	// Name returns the human-readable name of this module
	Name() string

	// Description returns a description of what this module does
	Description() string

	// Capabilities returns the list of capabilities this module provides
	Capabilities() []ModuleCapability

	// ValidateNodeType checks if a node type is valid for this module
	ValidateNodeType(nodeType string) bool

	// ValidateLinkType checks if a link type is valid for this module
	ValidateLinkType(linkType string) bool

	// ValidateMetadata validates module-specific metadata
	ValidateMetadata(meta map[string]interface{}) error
}
