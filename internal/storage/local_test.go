package storage_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"screws-box/internal/storage"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStorage(t *testing.T) *storage.LocalStorage {
	t.Helper()
	return storage.NewLocalStorage(t.TempDir())
}

func TestStoreRetrieveRoundTrip(t *testing.T) {
	ls := newTestStorage(t)
	ctx := context.Background()

	data := make([]byte, 1024)
	_, err := rand.Read(data)
	require.NoError(t, err)

	err = ls.Store(ctx, "a1b2c3d4", ".jpg", bytes.NewReader(data))
	require.NoError(t, err)

	pf, err := ls.Retrieve(ctx, "a1b2c3d4", ".jpg")
	require.NoError(t, err)
	defer pf.Reader.Close()

	got, err := io.ReadAll(pf.Reader)
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestRetrieveMetadata(t *testing.T) {
	ls := newTestStorage(t)
	ctx := context.Background()

	data := []byte("fake png data for testing")
	err := ls.Store(ctx, "b2c3d4e5", ".png", bytes.NewReader(data))
	require.NoError(t, err)

	pf, err := ls.Retrieve(ctx, "b2c3d4e5", ".png")
	require.NoError(t, err)
	defer pf.Reader.Close()

	assert.Equal(t, "image/png", pf.ContentType)
	assert.Equal(t, int64(len(data)), pf.Size)
}

func TestRetrieveNotFound(t *testing.T) {
	ls := newTestStorage(t)
	ctx := context.Background()

	_, err := ls.Retrieve(ctx, "nonexistent", ".jpg")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestDeleteRemovesFile(t *testing.T) {
	ls := newTestStorage(t)
	ctx := context.Background()

	err := ls.Store(ctx, "c3d4e5f6", ".jpg", strings.NewReader("test data"))
	require.NoError(t, err)

	err = ls.Delete(ctx, "c3d4e5f6", ".jpg")
	require.NoError(t, err)

	exists, err := ls.Exists(ctx, "c3d4e5f6", ".jpg")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestDeleteIdempotent(t *testing.T) {
	ls := newTestStorage(t)
	ctx := context.Background()

	err := ls.Delete(ctx, "nonexistent", ".jpg")
	assert.NoError(t, err)
}

func TestExists(t *testing.T) {
	ls := newTestStorage(t)
	ctx := context.Background()

	exists, err := ls.Exists(ctx, "d4e5f6g7", ".jpg")
	require.NoError(t, err)
	assert.False(t, exists)

	err = ls.Store(ctx, "d4e5f6g7", ".jpg", strings.NewReader("data"))
	require.NoError(t, err)

	exists, err = ls.Exists(ctx, "d4e5f6g7", ".jpg")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestDirectoryStructure(t *testing.T) {
	ls := newTestStorage(t)
	ctx := context.Background()
	baseDir := ls.BasePath()

	err := ls.Store(ctx, "a1b2c3d4", ".jpg", strings.NewReader("data"))
	require.NoError(t, err)

	expectedPath := filepath.Join(baseDir, "a1", "a1b2c3d4.jpg")
	_, err = os.Stat(expectedPath)
	assert.NoError(t, err, "expected file at %s", expectedPath)
}

func TestStoreThumbnail(t *testing.T) {
	ls := newTestStorage(t)
	ctx := context.Background()
	baseDir := ls.BasePath()

	err := ls.StoreThumbnail(ctx, "a1b2c3d4", ".jpg", strings.NewReader("thumb data"))
	require.NoError(t, err)

	expectedPath := filepath.Join(baseDir, "a1", "a1b2c3d4_thumb.jpg")
	_, err = os.Stat(expectedPath)
	assert.NoError(t, err, "expected thumbnail at %s", expectedPath)
}

func TestRetrieveThumbnail(t *testing.T) {
	ls := newTestStorage(t)
	ctx := context.Background()

	data := []byte("thumbnail content")
	err := ls.StoreThumbnail(ctx, "e5f6g7h8", ".jpg", bytes.NewReader(data))
	require.NoError(t, err)

	pf, err := ls.RetrieveThumbnail(ctx, "e5f6g7h8", ".jpg")
	require.NoError(t, err)
	defer pf.Reader.Close()

	got, err := io.ReadAll(pf.Reader)
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestAtomicWriteCleanup(t *testing.T) {
	ls := newTestStorage(t)
	ctx := context.Background()
	baseDir := ls.BasePath()

	err := ls.Store(ctx, "f6g7h8i9", ".jpg", strings.NewReader("data"))
	require.NoError(t, err)

	// Walk the storage directory and check for .tmp files
	var tmpFiles []string
	err = filepath.Walk(baseDir, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".tmp") {
			tmpFiles = append(tmpFiles, path)
		}
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, tmpFiles, "found leftover .tmp files: %v", tmpFiles)
}

func TestDeleteRemovesBothFiles(t *testing.T) {
	ls := newTestStorage(t)
	ctx := context.Background()

	err := ls.Store(ctx, "g7h8i9j0", ".jpg", strings.NewReader("main data"))
	require.NoError(t, err)
	err = ls.StoreThumbnail(ctx, "g7h8i9j0", ".jpg", strings.NewReader("thumb data"))
	require.NoError(t, err)

	err = ls.Delete(ctx, "g7h8i9j0", ".jpg")
	require.NoError(t, err)

	exists, err := ls.Exists(ctx, "g7h8i9j0", ".jpg")
	require.NoError(t, err)
	assert.False(t, exists, "main file should be deleted")

	// Verify thumbnail is also gone from disk
	baseDir := ls.BasePath()
	thumbPath := filepath.Join(baseDir, "g7", "g7h8i9j0_thumb.jpg")
	_, err = os.Stat(thumbPath)
	assert.True(t, os.IsNotExist(err), "thumbnail should be deleted")
}

func TestEmptyExtensionFallback(t *testing.T) {
	ls := newTestStorage(t)
	ctx := context.Background()

	err := ls.Store(ctx, "h8i9j0k1", "", strings.NewReader("binary data"))
	require.NoError(t, err)

	pf, err := ls.Retrieve(ctx, "h8i9j0k1", "")
	require.NoError(t, err)
	defer pf.Reader.Close()

	assert.Equal(t, "application/octet-stream", pf.ContentType)
}
