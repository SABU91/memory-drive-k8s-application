// Package models defines the core data types shared across the application.
package models

import "time"

// Kind describes what sort of item was uploaded.
type Kind string

const (
	KindNote  Kind = "note"  // A plain text note typed directly by the user.
	KindText  Kind = "text"  // An uploaded text file.
	KindImage Kind = "image" // An uploaded (small) image.
)

// File is a single stored item and its metadata.
// Binary blobs (text files, images) live on the mounted volume; only metadata
// and searchable text content are persisted in SQLite.
type File struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Kind        Kind      `json:"kind"`
	ContentType string    `json:"contentType"`
	Size        int64     `json:"size"`
	StoragePath string    `json:"-"`                 // absolute path on disk, hidden from API
	TextContent string    `json:"content,omitempty"` // inline text for notes / previews and search
	CreatedAt   time.Time `json:"createdAt"`
}
