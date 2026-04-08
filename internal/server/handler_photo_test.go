package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"screws-box/internal/model"
	"screws-box/internal/session"
	"screws-box/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test fixtures ---

func createTestJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := range 10 {
		for x := range 10 {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	buf := new(bytes.Buffer)
	require.NoError(t, jpeg.Encode(buf, img, nil))
	return buf.Bytes()
}

func createTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := range 10 {
		for x := range 10 {
			img.Set(x, y, color.RGBA{R: 0, G: 0, B: 255, A: 255})
		}
	}
	buf := new(bytes.Buffer)
	require.NoError(t, png.Encode(buf, img))
	return buf.Bytes()
}

func buildMultipartRequest(t *testing.T, fieldName, filename string, data []byte, extraFields map[string]string) *http.Request {
	t.Helper()
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, filename)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	for k, v := range extraFields {
		require.NoError(t, writer.WriteField(k, v))
	}
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/api/photos/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// --- Photo mock store ---

type photoMockStore struct {
	mockStore
	insertPhotoFn            func(ctx context.Context, p *model.Photo) error
	getPhotoByUUIDFn         func(ctx context.Context, uuid string) (*model.Photo, error)
	deletePhotoFn            func(ctx context.Context, uuid string) error
	listAllPhotosFn          func(ctx context.Context) ([]model.Photo, error)
	isPhotosEnabledFn        func(ctx context.Context) (bool, error)
	setPhotosEnabledFn       func(ctx context.Context, enabled bool) error
	getThumbnailSizeFn       func(ctx context.Context) (int, error)
	setThumbnailSizeFn       func(ctx context.Context, size int) error
	unlinkPhotoFn            func(ctx context.Context, uuid string) error
	updatePhotoCropModeFn    func(ctx context.Context, uuid, mode string) error
	linkPhotoToItemFn        func(ctx context.Context, itemID, photoID int64) error
	unlinkPhotoFromItemFn    func(ctx context.Context, itemID int64) error
	getItemsByPhotoIDFn      func(ctx context.Context, photoID int64) ([]model.ItemLinkInfo, error)
	countItemsByPhotoIDFn    func(ctx context.Context, photoID int64) (int, error)
	listAllPhotosWithLinksFn func(ctx context.Context) ([]model.PhotoWithLinks, error)
}

func (m *photoMockStore) InsertPhoto(ctx context.Context, p *model.Photo) error {
	if m.insertPhotoFn != nil {
		return m.insertPhotoFn(ctx, p)
	}
	return nil
}
func (m *photoMockStore) GetPhotoByUUID(ctx context.Context, uuid string) (*model.Photo, error) {
	if m.getPhotoByUUIDFn != nil {
		return m.getPhotoByUUIDFn(ctx, uuid)
	}
	return nil, fmt.Errorf("photo not found")
}
func (m *photoMockStore) DeletePhoto(ctx context.Context, uuid string) error {
	if m.deletePhotoFn != nil {
		return m.deletePhotoFn(ctx, uuid)
	}
	return nil
}
func (m *photoMockStore) ListAllPhotos(ctx context.Context) ([]model.Photo, error) {
	if m.listAllPhotosFn != nil {
		return m.listAllPhotosFn(ctx)
	}
	return nil, nil
}
func (m *photoMockStore) IsPhotosEnabled(ctx context.Context) (bool, error) {
	if m.isPhotosEnabledFn != nil {
		return m.isPhotosEnabledFn(ctx)
	}
	return true, nil
}
func (m *photoMockStore) SetPhotosEnabled(ctx context.Context, enabled bool) error {
	if m.setPhotosEnabledFn != nil {
		return m.setPhotosEnabledFn(ctx, enabled)
	}
	return nil
}
func (m *photoMockStore) GetThumbnailSize(ctx context.Context) (int, error) {
	if m.getThumbnailSizeFn != nil {
		return m.getThumbnailSizeFn(ctx)
	}
	return 200, nil
}
func (m *photoMockStore) SetThumbnailSize(ctx context.Context, size int) error {
	if m.setThumbnailSizeFn != nil {
		return m.setThumbnailSizeFn(ctx, size)
	}
	return nil
}
func (m *photoMockStore) UnlinkPhoto(ctx context.Context, uuid string) error {
	if m.unlinkPhotoFn != nil {
		return m.unlinkPhotoFn(ctx, uuid)
	}
	return nil
}
func (m *photoMockStore) UpdatePhotoCropMode(ctx context.Context, uuid, mode string) error {
	if m.updatePhotoCropModeFn != nil {
		return m.updatePhotoCropModeFn(ctx, uuid, mode)
	}
	return nil
}
func (m *photoMockStore) GetPhotoByItemID(_ context.Context, _ int64) (*model.Photo, error) {
	return nil, nil
}
func (m *photoMockStore) LinkPhotoToItem(ctx context.Context, itemID, photoID int64) error {
	if m.linkPhotoToItemFn != nil {
		return m.linkPhotoToItemFn(ctx, itemID, photoID)
	}
	return nil
}
func (m *photoMockStore) UnlinkPhotoFromItem(ctx context.Context, itemID int64) error {
	if m.unlinkPhotoFromItemFn != nil {
		return m.unlinkPhotoFromItemFn(ctx, itemID)
	}
	return nil
}
func (m *photoMockStore) GetItemsByPhotoID(ctx context.Context, photoID int64) ([]model.ItemLinkInfo, error) {
	if m.getItemsByPhotoIDFn != nil {
		return m.getItemsByPhotoIDFn(ctx, photoID)
	}
	return nil, nil
}
func (m *photoMockStore) CountItemsByPhotoID(ctx context.Context, photoID int64) (int, error) {
	if m.countItemsByPhotoIDFn != nil {
		return m.countItemsByPhotoIDFn(ctx, photoID)
	}
	return 0, nil
}
func (m *photoMockStore) ListAllPhotosWithLinks(ctx context.Context) ([]model.PhotoWithLinks, error) {
	if m.listAllPhotosWithLinksFn != nil {
		return m.listAllPhotosWithLinksFn(ctx)
	}
	return nil, nil
}

// --- Mock photo storage ---

type mockPhotoStorage struct {
	mu    sync.Mutex
	files map[string][]byte
}

func newMockPhotoStorage() *mockPhotoStorage {
	return &mockPhotoStorage{files: make(map[string][]byte)}
}

func (m *mockPhotoStorage) Store(_ context.Context, uuid, ext string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[uuid+ext] = data
	return nil
}

func (m *mockPhotoStorage) StoreThumbnail(_ context.Context, uuid, ext string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[uuid+"_thumb"+ext] = data
	return nil
}

func (m *mockPhotoStorage) Retrieve(_ context.Context, uuid, ext string) (*storage.PhotoFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[uuid+ext]
	if !ok {
		return nil, storage.ErrNotFound
	}
	ct := "image/jpeg"
	if ext == ".png" {
		ct = "image/png"
	}
	return &storage.PhotoFile{
		Reader:      io.NopCloser(bytes.NewReader(data)),
		ContentType: ct,
		Size:        int64(len(data)),
	}, nil
}

func (m *mockPhotoStorage) RetrieveThumbnail(_ context.Context, uuid, ext string) (*storage.PhotoFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[uuid+"_thumb"+ext]
	if !ok {
		return nil, storage.ErrNotFound
	}
	ct := "image/jpeg"
	if ext == ".png" {
		ct = "image/png"
	}
	return &storage.PhotoFile{
		Reader:      io.NopCloser(bytes.NewReader(data)),
		ContentType: ct,
		Size:        int64(len(data)),
	}, nil
}

func (m *mockPhotoStorage) Delete(_ context.Context, uuid, ext string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, uuid+ext)
	delete(m.files, uuid+"_thumb"+ext)
	return nil
}

func (m *mockPhotoStorage) Exists(_ context.Context, uuid, ext string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.files[uuid+ext]
	return ok, nil
}

// --- Test helper: create server with photo mocks ---

func setupPhotoTestServer(t *testing.T, ms *photoMockStore, ps storage.PhotoStorage) *Server {
	t.Helper()
	// Set up auth settings to disable auth (allow all requests through).
	if ms.getAuthSettingsFn == nil {
		ms.getAuthSettingsFn = func(_ context.Context) (*model.AuthSettings, error) {
			return &model.AuthSettings{Enabled: false}, nil
		}
	}
	// Set up grid data to avoid nil panics if grid routes are hit.
	if ms.getGridDataFn == nil {
		ms.getGridDataFn = func() (*model.GridData, error) {
			return &model.GridData{Rows: 1, Cols: 1}, nil
		}
	}
	memStore := session.NewMemoryStore(1*time.Hour, 10*time.Minute)
	t.Cleanup(func() { memStore.Close() })
	mgr := session.NewManager(memStore, 1*time.Hour, "Memory")
	return NewServer(ms, mgr, ps)
}

// --- Tests ---

func TestPhotoUploadJPEG(t *testing.T) {
	ms := &photoMockStore{}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)

	jpegData := createTestJPEG(t)
	req := buildMultipartRequest(t, "photo", "test.jpg", jpegData, nil)
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var photo model.Photo
	require.NoError(t, json.NewDecoder(w.Body).Decode(&photo))
	assert.NotEmpty(t, photo.UUID)
	assert.NotEmpty(t, photo.ThumbURL)
	assert.NotEmpty(t, photo.FullURL)
	assert.Contains(t, photo.ThumbURL, "/api/photos/")
	assert.Contains(t, photo.FullURL, "/api/photos/")
}

func TestPhotoUploadPNG(t *testing.T) {
	ms := &photoMockStore{}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)

	pngData := createTestPNG(t)
	req := buildMultipartRequest(t, "photo", "test.png", pngData, nil)
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	var photo model.Photo
	require.NoError(t, json.NewDecoder(w.Body).Decode(&photo))
	assert.NotEmpty(t, photo.UUID)
	assert.Equal(t, ".png", photo.Ext)
}

func TestPhotoUploadWithItemID(t *testing.T) {
	ms := &photoMockStore{}
	var insertedPhoto *model.Photo
	ms.insertPhotoFn = func(_ context.Context, p *model.Photo) error {
		p.ID = 99 // simulate DB setting ID
		insertedPhoto = p
		return nil
	}
	var linkedItemID, linkedPhotoID int64
	ms.linkPhotoToItemFn = func(_ context.Context, itemID, photoID int64) error {
		linkedItemID = itemID
		linkedPhotoID = photoID
		return nil
	}
	var unlinkedItemID int64
	ms.unlinkPhotoFromItemFn = func(_ context.Context, itemID int64) error {
		unlinkedItemID = itemID
		return nil
	}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)

	jpegData := createTestJPEG(t)
	req := buildMultipartRequest(t, "photo", "test.jpg", jpegData, map[string]string{"item_id": "42"})
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())
	require.NotNil(t, insertedPhoto)
	assert.Equal(t, int64(42), unlinkedItemID, "should unlink existing photo from item first")
	assert.Equal(t, int64(42), linkedItemID, "should auto-link to item 42")
	assert.Equal(t, int64(99), linkedPhotoID, "should link the newly inserted photo")
}

func TestPhotoUploadOversized(t *testing.T) {
	ms := &photoMockStore{}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)

	// Create a body that exceeds 10MB
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("photo", "huge.jpg")
	require.NoError(t, err)
	// Write 11MB of data
	_, err = part.Write(make([]byte, 11<<20))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/photos/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code, "body: %s", w.Body.String())
}

func TestPhotoUploadInvalidType(t *testing.T) {
	ms := &photoMockStore{}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)

	// Upload a plain text file
	req := buildMultipartRequest(t, "photo", "readme.txt", []byte("Hello, world!"), nil)
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "JPEG and PNG")
}

func TestPhotoServeFullPhoto(t *testing.T) {
	ms := &photoMockStore{}
	ps := newMockPhotoStorage()

	testUUID := "test-uuid-1234"
	testPhoto := &model.Photo{
		ID:   1,
		UUID: testUUID,
		Ext:  ".jpg",
	}
	ms.getPhotoByUUIDFn = func(_ context.Context, uuid string) (*model.Photo, error) {
		if uuid == testUUID {
			return testPhoto, nil
		}
		return nil, fmt.Errorf("not found")
	}

	// Store test data
	testData := createTestJPEG(t)
	require.NoError(t, ps.Store(context.Background(), testUUID, ".jpg", bytes.NewReader(testData)))

	srv := setupPhotoTestServer(t, ms, ps)
	req := httptest.NewRequest(http.MethodGet, "/api/photos/"+testUUID+"/full", nil)
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/jpeg", w.Header().Get("Content-Type"))
	assert.Equal(t, "public, max-age=86400", w.Header().Get("Cache-Control"))
	assert.True(t, len(w.Body.Bytes()) > 0, "response body should not be empty")
}

func TestPhotoServeThumb(t *testing.T) {
	ms := &photoMockStore{}
	ps := newMockPhotoStorage()

	testUUID := "test-uuid-thumb"
	testPhoto := &model.Photo{
		ID:   1,
		UUID: testUUID,
		Ext:  ".jpg",
	}
	ms.getPhotoByUUIDFn = func(_ context.Context, uuid string) (*model.Photo, error) {
		if uuid == testUUID {
			return testPhoto, nil
		}
		return nil, fmt.Errorf("not found")
	}

	testData := createTestJPEG(t)
	require.NoError(t, ps.StoreThumbnail(context.Background(), testUUID, ".jpg", bytes.NewReader(testData)))

	srv := setupPhotoTestServer(t, ms, ps)
	req := httptest.NewRequest(http.MethodGet, "/api/photos/"+testUUID+"/thumb", nil)
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "public, max-age=86400", w.Header().Get("Cache-Control"))
}

func TestPhotoServeNotFound(t *testing.T) {
	ms := &photoMockStore{}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)

	req := httptest.NewRequest(http.MethodGet, "/api/photos/nonexistent/full", nil)
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPhotoEndpointsDisabledReturns404(t *testing.T) {
	ms := &photoMockStore{}
	ms.isPhotosEnabledFn = func(_ context.Context) (bool, error) {
		return false, nil
	}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)
	router := srv.Router()

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/photos/upload"},
		{http.MethodGet, "/api/photos/some-uuid/full"},
		{http.MethodGet, "/api/photos/some-uuid/thumb"},
		{http.MethodPost, "/api/photos/regenerate"},
		{http.MethodGet, "/api/photos"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusNotFound, w.Code, "expected 404 when photos disabled for %s %s", ep.method, ep.path)
		})
	}
}

func TestPhotoUploadValidatesMagicBytes(t *testing.T) {
	ms := &photoMockStore{}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)

	// Create a file with .jpg extension but non-image content
	fakeJPEG := []byte("This is definitely not a JPEG file despite the extension")
	req := buildMultipartRequest(t, "photo", "fake.jpg", fakeJPEG, nil)
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "JPEG and PNG")
}

func TestPhotoUnlinkFromItem(t *testing.T) {
	ms := &photoMockStore{}
	var unlinkedItemID int64
	ms.unlinkPhotoFromItemFn = func(_ context.Context, itemID int64) error {
		unlinkedItemID = itemID
		return nil
	}

	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)

	req := httptest.NewRequest(http.MethodDelete, "/api/items/42/photo", nil)
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, int64(42), unlinkedItemID)
}

func TestPhotoSetPhotosEnabled(t *testing.T) {
	ms := &photoMockStore{}
	var setTo bool
	ms.setPhotosEnabledFn = func(_ context.Context, enabled bool) error {
		setTo = enabled
		return nil
	}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)

	req := httptest.NewRequest(http.MethodPut, "/api/shelf/photos-enabled", strings.NewReader(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.True(t, setTo)

	var resp map[string]bool
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.True(t, resp["photos_enabled"])
}

func TestPhotoSetThumbnailSize(t *testing.T) {
	ms := &photoMockStore{}
	var setSize int
	ms.setThumbnailSizeFn = func(_ context.Context, size int) error {
		setSize = size
		return nil
	}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)

	req := httptest.NewRequest(http.MethodPut, "/api/shelf/thumbnail-size", strings.NewReader(`{"size":300}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.Equal(t, 300, setSize)
}

func TestPhotoSetThumbnailSizeOutOfRange(t *testing.T) {
	ms := &photoMockStore{}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)
	router := srv.Router()

	tests := []struct {
		name string
		body string
	}{
		{"too small", `{"size":50}`},
		{"too large", `{"size":500}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/shelf/thumbnail-size", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
		})
	}
}

// --- Phase 26: Junction table endpoint tests ---

func TestPhotoLinkCreatesJunction(t *testing.T) {
	ms := &photoMockStore{}
	var linkedItemID, linkedPhotoID int64
	ms.linkPhotoToItemFn = func(_ context.Context, itemID, photoID int64) error {
		linkedItemID = itemID
		linkedPhotoID = photoID
		return nil
	}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)

	body := strings.NewReader(`{"photo_id": 5}`)
	req := httptest.NewRequest(http.MethodPost, "/api/items/1/photo", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code, "body: %s", w.Body.String())
	assert.Equal(t, int64(1), linkedItemID)
	assert.Equal(t, int64(5), linkedPhotoID)
}

func TestPhotoLinkReplacesExisting(t *testing.T) {
	ms := &photoMockStore{}
	var unlinkCalled bool
	var unlinkItemID int64
	ms.unlinkPhotoFromItemFn = func(_ context.Context, itemID int64) error {
		unlinkCalled = true
		unlinkItemID = itemID
		return nil
	}
	var linkCalled bool
	ms.linkPhotoToItemFn = func(_ context.Context, _, _ int64) error {
		linkCalled = true
		// Verify unlink was called before link.
		assert.True(t, unlinkCalled, "unlinkPhotoFromItem should be called before linkPhotoToItem")
		return nil
	}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)

	body := strings.NewReader(`{"photo_id": 10}`)
	req := httptest.NewRequest(http.MethodPost, "/api/items/7/photo", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, unlinkCalled, "should call unlinkPhotoFromItem")
	assert.Equal(t, int64(7), unlinkItemID)
	assert.True(t, linkCalled, "should call linkPhotoToItem")
}

func TestPhotoUnlinkPreservesPhoto(t *testing.T) {
	ms := &photoMockStore{}
	var unlinkCalled bool
	ms.unlinkPhotoFromItemFn = func(_ context.Context, itemID int64) error {
		unlinkCalled = true
		assert.Equal(t, int64(42), itemID)
		return nil
	}
	deletePhotoCalled := false
	ms.deletePhotoFn = func(_ context.Context, _ string) error {
		deletePhotoCalled = true
		return nil
	}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)

	req := httptest.NewRequest(http.MethodDelete, "/api/items/42/photo", nil)
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, unlinkCalled, "should call unlinkPhotoFromItem")
	assert.False(t, deletePhotoCalled, "should NOT call deletePhoto -- unlink preserves photo record")
}

func TestPhotoListForPicker(t *testing.T) {
	ms := &photoMockStore{}
	ms.listAllPhotosWithLinksFn = func(_ context.Context) ([]model.PhotoWithLinks, error) {
		return []model.PhotoWithLinks{
			{
				Photo:       model.Photo{ID: 1, UUID: "uuid-1", ContentType: "image/jpeg", Ext: ".jpg"},
				LinkedItems: []model.ItemLinkInfo{{ID: 10, Name: "M6 Bolt"}},
			},
			{
				Photo: model.Photo{ID: 2, UUID: "uuid-2", ContentType: "image/png", Ext: ".png"},
			},
		}, nil
	}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)

	req := httptest.NewRequest(http.MethodGet, "/api/photos", nil)
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var photos []model.PhotoWithLinks
	require.NoError(t, json.NewDecoder(w.Body).Decode(&photos))
	require.Len(t, photos, 2)

	assert.Equal(t, "uuid-1", photos[0].UUID)
	require.Len(t, photos[0].LinkedItems, 1)
	assert.Equal(t, "M6 Bolt", photos[0].LinkedItems[0].Name)

	assert.Equal(t, "uuid-2", photos[1].UUID)
	assert.Empty(t, photos[1].LinkedItems)
}

func TestPhotoOldUnlinkEndpointRemoved(t *testing.T) {
	ms := &photoMockStore{}
	ms.getPhotoByUUIDFn = func(_ context.Context, uuid string) (*model.Photo, error) {
		return &model.Photo{ID: 1, UUID: uuid, Ext: ".jpg"}, nil
	}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)

	req := httptest.NewRequest(http.MethodDelete, "/api/photos/some-uuid/item", nil)
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	// Should be 404 or 405 since the route no longer exists.
	assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed,
		"old DELETE /api/photos/{uuid}/item should be gone, got %d", w.Code)
}

func TestPhotoUploadWithoutItemIDNoLink(t *testing.T) {
	ms := &photoMockStore{}
	linkCalled := false
	ms.linkPhotoToItemFn = func(_ context.Context, _, _ int64) error {
		linkCalled = true
		return nil
	}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)

	jpegData := createTestJPEG(t)
	req := buildMultipartRequest(t, "photo", "test.jpg", jpegData, nil)
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())
	assert.False(t, linkCalled, "should NOT call linkPhotoToItem when no item_id provided")
}

func TestPhotoLinkSoftLimitWarning(t *testing.T) {
	ms := &photoMockStore{}
	ms.countItemsByPhotoIDFn = func(_ context.Context, _ int64) (int, error) {
		return 25, nil // exceed soft limit
	}
	var linkCalled bool
	ms.linkPhotoToItemFn = func(_ context.Context, _, _ int64) error {
		linkCalled = true
		return nil
	}
	ps := newMockPhotoStorage()
	srv := setupPhotoTestServer(t, ms, ps)

	body := strings.NewReader(`{"photo_id": 1}`)
	req := httptest.NewRequest(http.MethodPost, "/api/items/1/photo", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := srv.Router()
	router.ServeHTTP(w, req)

	// Should still succeed (soft limit warns but allows).
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, linkCalled, "should still link even when exceeding soft limit")
}
