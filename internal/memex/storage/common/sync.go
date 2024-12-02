package common

import (
	"sync"

	"memex/internal/memex/core"
)

// LockManager provides a centralized way to manage locks across the storage system
type LockManager struct {
	fileLock   sync.RWMutex // Lock for file operations
	indexLock  sync.RWMutex // Lock for index operations
	chunkLock  sync.RWMutex // Lock for chunk operations
	headerLock sync.RWMutex // Lock for header operations
}

// NewLockManager creates a new lock manager
func NewLockManager() *LockManager {
	return &LockManager{}
}

// WithFileLock executes a function while holding the file lock
func (lm *LockManager) WithFileLock(fn func() error) error {
	lm.fileLock.Lock()
	defer lm.fileLock.Unlock()
	return fn()
}

// WithFileRLock executes a function while holding the file read lock
func (lm *LockManager) WithFileRLock(fn func() error) error {
	lm.fileLock.RLock()
	defer lm.fileLock.RUnlock()
	return fn()
}

// WithIndexLock executes a function while holding the index lock
func (lm *LockManager) WithIndexLock(fn func() error) error {
	lm.indexLock.Lock()
	defer lm.indexLock.Unlock()
	return fn()
}

// WithIndexRLock executes a function while holding the index read lock
func (lm *LockManager) WithIndexRLock(fn func() error) error {
	lm.indexLock.RLock()
	defer lm.indexLock.RUnlock()
	return fn()
}

// WithChunkLock executes a function while holding the chunk lock
func (lm *LockManager) WithChunkLock(fn func() error) error {
	lm.chunkLock.Lock()
	defer lm.chunkLock.Unlock()
	return fn()
}

// WithChunkRLock executes a function while holding the chunk read lock
func (lm *LockManager) WithChunkRLock(fn func() error) error {
	lm.chunkLock.RLock()
	defer lm.chunkLock.RUnlock()
	return fn()
}

// WithChunkLockString executes a function while holding the chunk lock
func (lm *LockManager) WithChunkLockString(fn func() (string, error)) (string, error) {
	lm.chunkLock.Lock()
	defer lm.chunkLock.Unlock()
	return fn()
}

// WithChunkRLockBytes executes a function while holding the chunk read lock
func (lm *LockManager) WithChunkRLockBytes(fn func() ([]byte, error)) ([]byte, error) {
	lm.chunkLock.RLock()
	defer lm.chunkLock.RUnlock()
	return fn()
}

// WithChunkLockNode executes a function while holding the chunk lock
func (lm *LockManager) WithChunkLockNode(fn func() (*core.Node, error)) (*core.Node, error) {
	lm.chunkLock.Lock()
	defer lm.chunkLock.Unlock()
	return fn()
}

// WithChunkLockLinks executes a function while holding the chunk lock
func (lm *LockManager) WithChunkLockLinks(fn func() ([]*core.Link, error)) ([]*core.Link, error) {
	lm.chunkLock.Lock()
	defer lm.chunkLock.Unlock()
	return fn()
}

// WithHeaderLock executes a function while holding the header lock
func (lm *LockManager) WithHeaderLock(fn func() error) error {
	lm.headerLock.Lock()
	defer lm.headerLock.Unlock()
	return fn()
}

// WithHeaderRLock executes a function while holding the header read lock
func (lm *LockManager) WithHeaderRLock(fn func() error) error {
	lm.headerLock.RLock()
	defer lm.headerLock.RUnlock()
	return fn()
}
