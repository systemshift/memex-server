package test

import (
	"os"

	"github.com/systemshift/memex/internal/memex/core"
	"github.com/systemshift/memex/internal/memex/storage/common"
)

// MockStorage implements core/transaction.Storage for testing
type MockStorage struct {
	file *common.File
	path string
}

// NewMockStorage creates a new mock storage
func NewMockStorage(path string) (*MockStorage, error) {
	file, err := common.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	return &MockStorage{
		file: file,
		path: path,
	}, nil
}

// GetFile returns the underlying file
func (m *MockStorage) GetFile() interface{} {
	return m.file
}

// GetLockManager returns a lock manager
func (m *MockStorage) GetLockManager() interface{} {
	return nil // No locking needed for tests
}

// GetNode returns a mock node
func (m *MockStorage) GetNode(id string) (*core.Node, error) {
	return &core.Node{ID: id}, nil
}

// GetLinks returns mock links
func (m *MockStorage) GetLinks(nodeID string) ([]*core.Link, error) {
	return nil, nil
}

// Path returns the storage path
func (m *MockStorage) Path() string {
	return m.path
}

// Close closes the storage
func (m *MockStorage) Close() error {
	return m.file.Close()
}
