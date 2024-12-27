package migration

import (
	"archive/tar"
	"fmt"
	"io"
	"os"

	"github.com/systemshift/memex/internal/memex/core"
)

// VerifyRepository checks repository integrity
func VerifyRepository(repo core.Repository) error {
	// TODO: Implement repository verification
	// For now, just check if we can access the repository
	_, err := repo.GetNode("0")
	if err != nil && err.Error() != "node not found" {
		return fmt.Errorf("checking repository access: %w", err)
	}
	return nil
}

// VerifyArchive checks archive integrity
func VerifyArchive(path string) error {
	// Open archive
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening archive: %w", err)
	}
	defer file.Close()

	// Create tar reader
	tr := tar.NewReader(file)

	// Create importer to verify archive
	importer := NewImporter(nil, file, ImportOptions{})

	// Try to read manifest
	if _, err := importer.readManifest(tr); err != nil && err != io.EOF {
		return fmt.Errorf("reading manifest: %w", err)
	}

	return nil
}
