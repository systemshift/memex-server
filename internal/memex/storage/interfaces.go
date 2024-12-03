package storage

// Chunker splits content into chunks using a specific strategy
type Chunker interface {
	// Split divides content into chunks
	Split(content []byte) [][]byte
}

// Addresser generates addresses for chunks
type Addresser interface {
	// Address generates a unique address for a chunk
	Address(chunk []byte) []byte
}

// Store manages content storage and retrieval
type Store interface {
	// Put stores content and returns its chunk addresses
	Put(content []byte) ([][]byte, error)

	// Get retrieves content from its chunk addresses
	Get(addresses [][]byte) ([]byte, error)

	// Delete marks content as deleted
	Delete(addresses [][]byte) error

	// Close releases any resources
	Close() error
}

// Transaction represents a storage operation
type Transaction interface {
	// Begin starts a new transaction
	Begin() error

	// Commit finalizes the transaction
	Commit() error

	// Rollback undoes the transaction
	Rollback() error
}
