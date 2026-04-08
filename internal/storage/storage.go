package storage

import (
	"context"
	"errors"
	"io"
)

// ErrNotFound is returned when the requested photo file does not exist in storage.
var ErrNotFound = errors.New("photo not found")

// PhotoFile contains the file data and metadata needed to serve a photo.
type PhotoFile struct {
	Reader      io.ReadCloser
	ContentType string
	Size        int64
}

// PhotoStorage defines the interface for pluggable photo storage backends.
// The local filesystem backend (LocalStorage) is the default implementation.
// An S3-compatible backend can be added by implementing this interface.
type PhotoStorage interface {
	// Store writes photo data from r to storage using UUID-based naming.
	// Extension must include the leading dot (e.g., ".jpg", ".png").
	// The storage layer builds the full path internally.
	Store(ctx context.Context, uuid, ext string, r io.Reader) error

	// StoreThumbnail writes a thumbnail variant alongside the main photo.
	// Uses _thumb suffix in the filename.
	StoreThumbnail(ctx context.Context, uuid, ext string, r io.Reader) error

	// Retrieve returns the photo file with metadata. Caller MUST close Reader.
	// Returns ErrNotFound if the photo does not exist.
	Retrieve(ctx context.Context, uuid, ext string) (*PhotoFile, error)

	// RetrieveThumbnail returns the thumbnail variant. Caller MUST close Reader.
	// Returns ErrNotFound if the thumbnail does not exist.
	RetrieveThumbnail(ctx context.Context, uuid, ext string) (*PhotoFile, error)

	// Delete removes the photo and its thumbnail from storage.
	// Returns nil if the file does not exist (idempotent).
	Delete(ctx context.Context, uuid, ext string) error

	// Exists reports whether the photo file exists in storage.
	Exists(ctx context.Context, uuid, ext string) (bool, error)
}
