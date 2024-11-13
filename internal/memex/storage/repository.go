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
	obj := core.Object{
		Content:  content,
		Type:     contentType,
		Version:  1,
		Created:  time.Now(),
		Modified: time.Now(),
		Meta:     meta,
	}

	// Store object
	id, err := r.objects.Store(obj)
	if err != nil {
		return "", fmt.Errorf("storing object: %w", err)
	}

	// Store initial version
	err = r.versions.Store(id, 1, content, "Initial version")
	if err != nil {
		return "", fmt.Errorf("storing version: %w", err)
	}

	return id, nil
}

// Get retrieves an object by ID
func (r *Repository) Get(id string) (core.Object, error) {
	return r.objects.Load(id)
}

// Update updates an object's content
func (r *Repository) Update(id string, content []byte) error {
	// Get current object
	obj, err := r.objects.Load(id)
	if err != nil {
		return fmt.Errorf("loading object: %w", err)
	}

	// Update object
	obj.Content = content
	obj.Version++
	obj.Modified = time.Now()

	// Store updated object
	_, err = r.objects.Store(obj)
	if err != nil {
		return fmt.Errorf("storing updated object: %w", err)
	}

	// Store new version
	err = r.versions.Store(id, obj.Version, content, "Update")
	if err != nil {
		return fmt.Errorf("storing version: %w", err)
	}

	return nil
}

// Delete removes an object
func (r *Repository) Delete(id string) error {
	// Delete object
	if err := r.objects.Delete(id); err != nil {
		return fmt.Errorf("deleting object: %w", err)
	}

	// Delete versions
	if err := r.versions.Delete(id); err != nil {
		return fmt.Errorf("deleting versions: %w", err)
	}

	// Delete related links
	links := r.links.GetLinked(id)
	for _, linkedID := range links {
		r.links.Delete(id, linkedID)
		r.links.Delete(linkedID, id)
	}

	return nil
}

// Link creates a link between objects
func (r *Repository) Link(source, target string, linkType string, meta map[string]any) error {
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
	outgoing := r.links.GetBySource(id)
	incoming := r.links.GetByTarget(id)
	return append(outgoing, incoming...), nil
}

// GetVersion retrieves a specific version of an object
func (r *Repository) GetVersion(id string, version int) (core.Object, error) {
	// Get current object for metadata
	obj, err := r.objects.Load(id)
	if err != nil {
		return obj, fmt.Errorf("loading object: %w", err)
	}

	// Get version content
	content, err := r.versions.Load(id, version)
	if err != nil {
		return obj, fmt.Errorf("loading version: %w", err)
	}

	// Get version info
	info, err := r.versions.GetInfo(id, version)
	if err != nil {
		return obj, fmt.Errorf("loading version info: %w", err)
	}

	// Update object with version info
	obj.Content = content
	obj.Version = info.Version
	obj.Modified = info.Created

	return obj, nil
}

// ListVersions returns all versions of an object
func (r *Repository) ListVersions(id string) ([]int, error) {
	versions, err := r.versions.List(id)
	if err != nil {
		return nil, fmt.Errorf("listing versions: %w", err)
	}

	var nums []int
	for _, v := range versions {
		var num int
		fmt.Sscanf(v, "v%d", &num)
		nums = append(nums, num)
	}

	return nums, nil
}

// List returns all object IDs
func (r *Repository) List() []string {
	return r.objects.List()
}

// FindByType returns objects of a specific type
func (r *Repository) FindByType(contentType string) []core.Object {
	var results []core.Object
	for _, id := range r.List() {
		obj, err := r.Get(id)
		if err != nil {
			continue
		}
		if obj.Type == contentType {
			results = append(results, obj)
		}
	}
	return results
}

// deepEqual compares two values, handling slices and maps
func deepEqual(a, b interface{}) bool {
	if a == nil || b == nil {
		return a == b
	}

	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)

	if va.Kind() != vb.Kind() {
		return false
	}

	switch va.Kind() {
	case reflect.Slice:
		if va.Len() != vb.Len() {
			return false
		}
		for i := 0; i < va.Len(); i++ {
			if !deepEqual(va.Index(i).Interface(), vb.Index(i).Interface()) {
				return false
			}
		}
		return true
	case reflect.Map:
		if va.Len() != vb.Len() {
			return false
		}
		for _, k := range va.MapKeys() {
			va1 := va.MapIndex(k)
			vb1 := vb.MapIndex(k)
			if !vb1.IsValid() || !deepEqual(va1.Interface(), vb1.Interface()) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}

// Search finds objects matching metadata criteria
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
			if objVal, ok := obj.Meta[k]; !ok || !deepEqual(objVal, v) {
				matches = false
				break
			}
		}
		if matches {
			results = append(results, obj)
		}
	}
	return results
}
