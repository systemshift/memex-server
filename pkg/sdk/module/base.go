package module

import (
	"memex/pkg/sdk/types"
)

// BaseModule provides a basic implementation of the Module interface
type BaseModule struct {
	id          string
	name        string
	description string
}

// NewBaseModule creates a new base module
func NewBaseModule(id, name, description string) *BaseModule {
	return &BaseModule{
		id:          id,
		name:        name,
		description: description,
	}
}

// ID returns the module identifier
func (m *BaseModule) ID() string {
	return m.id
}

// Name returns the module name
func (m *BaseModule) Name() string {
	return m.name
}

// Description returns the module description
func (m *BaseModule) Description() string {
	return m.description
}

// Capabilities returns an empty list of capabilities by default
func (m *BaseModule) Capabilities() []types.ModuleCapability {
	return nil
}

// ValidateNodeType returns true by default
func (m *BaseModule) ValidateNodeType(nodeType string) bool {
	return true
}

// ValidateLinkType returns true by default
func (m *BaseModule) ValidateLinkType(linkType string) bool {
	return true
}

// ValidateMetadata returns nil by default
func (m *BaseModule) ValidateMetadata(meta map[string]interface{}) error {
	return nil
}
