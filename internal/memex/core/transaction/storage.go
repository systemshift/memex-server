package transaction

import "memex/internal/memex/core"

// Storage defines the interface that transaction package needs from storage
type Storage interface {
	// Path returns the repository path
	Path() string

	// Node operations
	GetNode(id string) (*core.Node, error)
	GetLinks(nodeID string) ([]*core.Link, error)

	// File operations
	GetFile() interface{}        // Returns the underlying file handle
	GetLockManager() interface{} // Returns the lock manager
}

// StorageProvider is implemented by types that can provide Storage access
type StorageProvider interface {
	Storage() Storage
}
