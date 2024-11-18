package storage

import (
	"fmt"
	"reflect"
	"time"

	"memex/internal/memex/core"
)

// Repository implements the core.Repository interface
type Repository struct {
	objects  *BinaryStore
	versions *BinaryVersionStore
	links    *BinaryLinkStore
	rootDir  string
}

// NewRepository creates a new repository instance
func NewRepository(path string) (*Repository, error) {
	// Initialize storage components
	objects, err := NewBinaryStore(path)
	if err != nil {
		return nil, fmt.Errorf("initializing object store: %w", err)
	}

	versions, err := NewVersionStore(path)
	if err != nil {
		return nil, fmt.Errorf("initializing version store: %w", err)
	}

	links, err := NewLinkStore(path)
	if err != nil {
		return nil, fmt.Errorf("initializing link store: %w", err)
	}

	return &Repository{
		objects:  objects,
		versions: versions,
		links:    links,
		rootDir:  path,
	}, nil
}

// Init initializes a new repository
func (r *Repository) Init(path string) error {
	// Create new repository instance
	newRepo, err := NewRepository(path)
	if err != nil {
		return fmt.Errorf("creating repository: %w", err)
	}

	// Copy values
	*r = *newRepo
	return nil
}

// Open opens an existing repository
func (r *Repository) Open(path string) error {
	return r.Init(path) // Same as Init for now
}

// Close closes the repository
func (r *Repository) Close() error {
	// Nothing to do for now
	return nil
}

// Add adds new content to the repository
func (r *Repository) Add(content []byte, contentType string, meta map[string]any) (string, error) {
	// Split content into chunks
	chunks, err := ChunkContent(content)
	if err != nil {
		return "", fmt.Errorf("chunking content: %w", err)
	}

	// Store each chunk
	var chunkHashes []string
	for _, chunk := range chunks {
		if err := r.objects.StoreChunk(chunk.Hash, chunk.Content); err != nil {
			return "", fmt.Errorf("storing chunk: %w", err)
		}
		chunkHashes = append(chunkHashes, chunk.Hash)
	}

	// Create object
	obj := core.Object{
		Type:     contentType,
		Version:  1,
		Created:  time.Now(),
		Modified: time.Now(),
		Meta:     meta,
		Chunks:   chunkHashes,
		Content:  content, // Store original content for backward compatibility
	}

	// Store object
	id, err := r.objects.Store(obj)
	if err != nil {
		return "", fmt.Errorf("storing object: %w", err)
	}

	// Store initial version
	if err := r.versions.Store(id, 1, chunkHashes); err != nil {
		return "", fmt.Errorf("storing version: %w", err)
	}

	return id, nil
}

// Get retrieves an object by ID
func (r *Repository) Get(id string) (core.Object, error) {
	obj, err := r.objects.Load(id)
	if err != nil {
		return obj, fmt.Errorf("loading object: %w", err)
	}

	// If object has chunks but no content, reassemble it
	if len(obj.Chunks) > 0 && len(obj.Content) == 0 {
		var chunks []Chunk
		for _, hash := range obj.Chunks {
			content, err := r.objects.LoadChunk(hash)
			if err != nil {
				return obj, fmt.Errorf("loading chunk %s: %w", hash, err)
			}
			chunks = append(chunks, Chunk{Hash: hash, Content: content})
		}
		obj.Content = ReassembleContent(chunks)
	}

	return obj, nil
}

// Update updates an object's content
func (r *Repository) Update(id string, content []byte) error {
	// Get current object
	obj, err := r.objects.Load(id)
	if err != nil {
		return fmt.Errorf("loading object: %w", err)
	}

	// Split new content into chunks
	chunks, err := ChunkContent(content)
	if err != nil {
		return fmt.Errorf("chunking content: %w", err)
	}

	// Store each chunk
	var chunkHashes []string
	for _, chunk := range chunks {
		if err := r.objects.StoreChunk(chunk.Hash, chunk.Content); err != nil {
			return fmt.Errorf("storing chunk: %w", err)
		}
		chunkHashes = append(chunkHashes, chunk.Hash)
	}

	// Update object
	obj.Chunks = chunkHashes
	obj.Content = content // Update raw content for backward compatibility
	obj.Version++
	obj.Modified = time.Now()

	// Store updated object
	if _, err = r.objects.Store(obj); err != nil {
		return fmt.Errorf("storing updated object: %w", err)
	}

	// Store new version
	if err := r.versions.Store(id, obj.Version, chunkHashes); err != nil {
		return fmt.Errorf("storing version: %w", err)
	}

	return nil
}

// Delete removes an object and its chunks
func (r *Repository) Delete(id string) error {
	// Get object first to get its chunks
	obj, err := r.objects.Load(id)
	if err != nil {
		return fmt.Errorf("loading object: %w", err)
	}

	// Delete all chunks
	for _, hash := range obj.Chunks {
		if err := r.objects.DeleteChunk(hash); err != nil {
			return fmt.Errorf("deleting chunk %s: %w", hash, err)
		}
	}

	// Delete all versions
	if err := r.versions.Delete(id); err != nil {
		return fmt.Errorf("deleting versions: %w", err)
	}

	// Delete all links
	if err := r.links.Delete(id, ""); err != nil {
		return fmt.Errorf("deleting links: %w", err)
	}

	// Delete the object itself
	if err := r.objects.Delete(id); err != nil {
		return fmt.Errorf("deleting object: %w", err)
	}

	return nil
}

// Link creates a link between objects
func (r *Repository) Link(source, target, linkType string, meta map[string]any) error {
	link := core.Link{
		Source: source,
		Target: target,
		Type:   linkType,
		Meta:   meta,
	}
	return r.links.Store(link)
}

// Unlink removes a link between objects
func (r *Repository) Unlink(source, target string) error {
	return r.links.Delete(source, target)
}

// GetLinks returns all links for an object
func (r *Repository) GetLinks(id string) ([]core.Link, error) {
	return r.links.GetBySource(id), nil
}

// List returns all object IDs
func (r *Repository) List() []string {
	return r.objects.List()
}

// FindByType returns all objects of a specific type
func (r *Repository) FindByType(contentType string) []core.Object {
	var objects []core.Object
	for _, id := range r.List() {
		obj, err := r.Get(id)
		if err != nil {
			continue
		}
		if obj.Type == contentType {
			objects = append(objects, obj)
		}
	}
	return objects
}

// Search finds objects matching criteria
func (r *Repository) Search(query map[string]any) []core.Object {
	var results []core.Object
	for _, id := range r.List() {
		obj, err := r.Get(id)
		if err != nil {
			continue
		}
		// Check if object matches query
		matches := true
		for k, v := range query {
			if objVal, ok := obj.Meta[k]; !ok {
				matches = false
				break
			} else {
				// Handle different types of values
				switch val := v.(type) {
				case []string:
					// For string arrays, check if any value matches
					if objArr, ok := objVal.([]string); ok {
						found := false
						for _, s := range val {
							for _, os := range objArr {
								if s == os {
									found = true
									break
								}
							}
							if found {
								break
							}
						}
						if !found {
							matches = false
							break
						}
					} else {
						matches = false
						break
					}
				default:
					// For other types, use reflect.DeepEqual
					if !reflect.DeepEqual(objVal, v) {
						matches = false
						break
					}
				}
			}
		}
		if matches {
			results = append(results, obj)
		}
	}
	return results
}

// GetChunk retrieves a chunk by hash
func (r *Repository) GetChunk(hash string) ([]byte, error) {
	return r.objects.LoadChunk(hash)
}

// GetObjectChunks retrieves all chunks for an object
func (r *Repository) GetObjectChunks(id string) ([][]byte, error) {
	obj, err := r.objects.Load(id)
	if err != nil {
		return nil, fmt.Errorf("loading object: %w", err)
	}

	var chunks [][]byte
	for _, hash := range obj.Chunks {
		content, err := r.objects.LoadChunk(hash)
		if err != nil {
			return nil, fmt.Errorf("loading chunk %s: %w", hash, err)
		}
		chunks = append(chunks, content)
	}

	return chunks, nil
}

// LinkChunks creates a link between specific chunks
func (r *Repository) LinkChunks(sourceID, sourceChunk, targetID, targetChunk, linkType string, meta map[string]any) error {
	// Verify chunks exist
	if _, err := r.GetChunk(sourceChunk); err != nil {
		return fmt.Errorf("source chunk not found: %w", err)
	}
	if _, err := r.GetChunk(targetChunk); err != nil {
		return fmt.Errorf("target chunk not found: %w", err)
	}

	// Create a new link with chunk references
	link := core.Link{
		Source:      sourceID,
		Target:      targetID,
		Type:        linkType,
		Meta:        meta,
		SourceChunk: sourceChunk,
		TargetChunk: targetChunk,
	}

	// Store the link
	return r.links.Store(link)
}

// GetVersion retrieves a specific version of an object
func (r *Repository) GetVersion(id string, version int) (core.Object, error) {
	// Get base object
	obj, err := r.objects.Load(id)
	if err != nil {
		return obj, fmt.Errorf("loading object: %w", err)
	}

	// Get version chunks
	chunkHashes, err := r.versions.Load(id, version)
	if err != nil {
		return obj, fmt.Errorf("loading version: %w", err)
	}

	// Load chunks
	var chunks []Chunk
	for _, hash := range chunkHashes {
		content, err := r.objects.LoadChunk(hash)
		if err != nil {
			return obj, fmt.Errorf("loading chunk %s: %w", hash, err)
		}
		chunks = append(chunks, Chunk{Hash: hash, Content: content})
	}

	// Update object with version data
	obj.Version = version
	obj.Chunks = chunkHashes
	obj.Content = ReassembleContent(chunks)

	return obj, nil
}

// ListVersions returns all versions of an object
func (r *Repository) ListVersions(id string) ([]int, error) {
	versions := r.versions.List(id)
	return versions, nil
}
