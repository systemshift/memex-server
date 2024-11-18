package memex

import (
	"fmt"
	"path/filepath"

	"memex/internal/memex/storage"
)

// Memex represents a content-addressable storage system
type Memex struct {
	repo *storage.Repository
}

// Open creates or opens a Memex instance at the specified path
func Open(path string) (*Memex, error) {
	repoPath := filepath.Join(path, ".memex")
	repo, err := storage.NewRepository(repoPath)
	if err != nil {
		return nil, fmt.Errorf("initializing repository: %w", err)
	}

	return &Memex{repo: repo}, nil
}

// Add adds new content to the repository
func (m *Memex) Add(content []byte, contentType string, meta map[string]any) (string, error) {
	return m.repo.Add(content, contentType, meta)
}

// Get retrieves an object by ID
func (m *Memex) Get(id string) (Object, error) {
	obj, err := m.repo.Get(id)
	if err != nil {
		return Object{}, err
	}
	return convertObject(obj), nil
}

// Delete removes an object and its chunks
func (m *Memex) Delete(id string) error {
	return m.repo.Delete(id)
}

// Update updates an object's content
func (m *Memex) Update(id string, content []byte) error {
	return m.repo.Update(id, content)
}

// Link creates a link between objects
func (m *Memex) Link(source, target, linkType string, meta map[string]any) error {
	return m.repo.Link(source, target, linkType, meta)
}

// LinkChunks creates a link between specific chunks
func (m *Memex) LinkChunks(sourceID, sourceChunk, targetID, targetChunk, linkType string, meta map[string]any) error {
	return m.repo.LinkChunks(sourceID, sourceChunk, targetID, targetChunk, linkType, meta)
}

// GetLinks returns all links for an object
func (m *Memex) GetLinks(id string) ([]Link, error) {
	coreLinks, err := m.repo.GetLinks(id)
	if err != nil {
		return nil, err
	}
	var links []Link
	for _, link := range coreLinks {
		links = append(links, convertLink(link))
	}
	return links, nil
}

// List returns all object IDs
func (m *Memex) List() []string {
	return m.repo.List()
}

// FindByType returns all objects of a specific type
func (m *Memex) FindByType(contentType string) []Object {
	coreObjs := m.repo.FindByType(contentType)
	var objects []Object
	for _, obj := range coreObjs {
		objects = append(objects, convertObject(obj))
	}
	return objects
}

// Search finds objects matching criteria
func (m *Memex) Search(query map[string]any) []Object {
	coreObjs := m.repo.Search(query)
	var objects []Object
	for _, obj := range coreObjs {
		objects = append(objects, convertObject(obj))
	}
	return objects
}

// GetChunk retrieves a chunk by hash
func (m *Memex) GetChunk(hash string) ([]byte, error) {
	return m.repo.GetChunk(hash)
}

// GetObjectChunks retrieves all chunks for an object
func (m *Memex) GetObjectChunks(id string) ([][]byte, error) {
	return m.repo.GetObjectChunks(id)
}

// GetVersion retrieves a specific version of an object
func (m *Memex) GetVersion(id string, version int) (Object, error) {
	obj, err := m.repo.GetVersion(id, version)
	if err != nil {
		return Object{}, err
	}
	return convertObject(obj), nil
}

// ListVersions returns all versions of an object
func (m *Memex) ListVersions(id string) ([]int, error) {
	return m.repo.ListVersions(id)
}
