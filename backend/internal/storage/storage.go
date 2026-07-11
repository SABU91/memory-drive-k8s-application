// Package storage persists uploaded blobs on the mounted volume.
package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Store writes and reads file blobs under a single base directory.
type Store struct {
	baseDir string
}

// New ensures the base directory exists and returns a Store.
func New(baseDir string) (*Store, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}
	return &Store{baseDir: baseDir}, nil
}

// Save streams src into a new file named after id and returns the number of
// bytes written and the absolute path on disk.
func (s *Store) Save(id string, src io.Reader) (int64, string, error) {
	path := s.pathFor(id)
	dst, err := os.Create(path)
	if err != nil {
		return 0, "", fmt.Errorf("create blob: %w", err)
	}
	defer dst.Close()

	n, err := io.Copy(dst, src)
	if err != nil {
		return 0, "", fmt.Errorf("write blob: %w", err)
	}
	return n, path, nil
}

// Open returns a readable handle to the blob for id. The caller must Close it.
func (s *Store) Open(id string) (*os.File, error) {
	return os.Open(s.pathFor(id))
}

// Delete removes the blob for id. Missing files are treated as success.
func (s *Store) Delete(id string) error {
	err := os.Remove(s.pathFor(id))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete blob: %w", err)
	}
	return nil
}

func (s *Store) pathFor(id string) string {
	return filepath.Join(s.baseDir, id)
}
