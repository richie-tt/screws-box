package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
)

// LocalStorage implements PhotoStorage using the local filesystem.
// Files are stored in hash-prefix subdirectories under basePath
// (e.g., basePath/a1/{uuid}.jpg).
type LocalStorage struct {
	basePath string
}

// NewLocalStorage creates a new LocalStorage instance.
// basePath is the root directory for photo storage (e.g., "./photos").
// Directories are created lazily on first write, not at construction time.
func NewLocalStorage(basePath string) *LocalStorage {
	return &LocalStorage{basePath: basePath}
}

// BasePath returns the root storage directory. Used by tests to verify directory structure.
func (ls *LocalStorage) BasePath() string {
	return ls.basePath
}

// filePath returns the full filesystem path for a photo file.
// Uses the first 2 characters of uuid as a hash-prefix subdirectory.
func (ls *LocalStorage) filePath(uuid, ext string) string {
	return filepath.Join(ls.basePath, uuid[:2], uuid+ext)
}

// thumbPath returns the full filesystem path for a thumbnail file.
func (ls *LocalStorage) thumbPath(uuid, ext string) string {
	return filepath.Join(ls.basePath, uuid[:2], uuid+"_thumb"+ext)
}

// atomicWrite writes data from r to the target path using a temp file and rename.
// On failure, the temp file is cleaned up to prevent half-written files.
func (ls *LocalStorage) atomicWrite(path string, r io.Reader) error {
	tmpPath := path + ".tmp"

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		slog.Error("storage: failed to create directory", "path", filepath.Dir(path), "error", err)
		return fmt.Errorf("create directory: %w", err)
	}

	f, err := os.Create(tmpPath)
	if err != nil {
		slog.Error("storage: failed to create temp file", "path", tmpPath, "error", err)
		return fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		os.Remove(tmpPath)
		slog.Error("storage: failed to write data", "path", tmpPath, "error", err)
		return fmt.Errorf("write data: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		slog.Error("storage: failed to close temp file", "path", tmpPath, "error", err)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		slog.Error("storage: failed to rename temp file", "from", tmpPath, "to", path, "error", err)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// retrieveFile opens a file and returns it as a PhotoFile with metadata.
func (ls *LocalStorage) retrieveFile(path, ext string) (*PhotoFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("stat file: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("open file: %w", err)
	}

	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return &PhotoFile{
		Reader:      f,
		ContentType: contentType,
		Size:        info.Size(),
	}, nil
}

// Store writes photo data from r to storage using UUID-based naming.
func (ls *LocalStorage) Store(_ context.Context, uuid, ext string, r io.Reader) error {
	return ls.atomicWrite(ls.filePath(uuid, ext), r)
}

// StoreThumbnail writes a thumbnail variant alongside the main photo.
func (ls *LocalStorage) StoreThumbnail(_ context.Context, uuid, ext string, r io.Reader) error {
	return ls.atomicWrite(ls.thumbPath(uuid, ext), r)
}

// Retrieve returns the photo file with metadata. Caller MUST close Reader.
func (ls *LocalStorage) Retrieve(_ context.Context, uuid, ext string) (*PhotoFile, error) {
	return ls.retrieveFile(ls.filePath(uuid, ext), ext)
}

// RetrieveThumbnail returns the thumbnail variant. Caller MUST close Reader.
func (ls *LocalStorage) RetrieveThumbnail(_ context.Context, uuid, ext string) (*PhotoFile, error) {
	return ls.retrieveFile(ls.thumbPath(uuid, ext), ext)
}

// Delete removes the photo and its thumbnail from storage.
// Returns nil if the files do not exist (idempotent).
func (ls *LocalStorage) Delete(_ context.Context, uuid, ext string) error {
	mainPath := ls.filePath(uuid, ext)
	if err := os.Remove(mainPath); err != nil && !os.IsNotExist(err) {
		slog.Error("storage: failed to delete file", "path", mainPath, "error", err)
		return fmt.Errorf("delete file: %w", err)
	}

	thumbPath := ls.thumbPath(uuid, ext)
	if err := os.Remove(thumbPath); err != nil && !os.IsNotExist(err) {
		slog.Error("storage: failed to delete thumbnail", "path", thumbPath, "error", err)
		return fmt.Errorf("delete thumbnail: %w", err)
	}

	return nil
}

// Exists reports whether the photo file exists in storage.
func (ls *LocalStorage) Exists(_ context.Context, uuid, ext string) (bool, error) {
	_, err := os.Stat(ls.filePath(uuid, ext))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("stat file: %w", err)
}
