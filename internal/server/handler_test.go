package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"screws-box/internal/model"
	oidcpkg "screws-box/internal/oidc"
	"screws-box/internal/session"
	"screws-box/internal/store"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	s := &store.Store{}
	tmpFile := filepath.Join(t.TempDir(), "test.db")
	require.NoError(t, s.Open(tmpFile))
	t.Cleanup(func() { s.Close() })

	memStore := session.NewMemoryStore(1*time.Hour, 10*time.Minute)
	t.Cleanup(func() { memStore.Close() })
	mgr := session.NewManager(memStore, 1*time.Hour, "Memory")

	srv := NewServer(s, mgr, "test")
	router := srv.Router()
	return router, s
}

func createTestItem(t *testing.T, router http.Handler) model.ItemResponse {
	t.Helper()
	body := `{"name":"Test Bolt","container_id":1,"tags":["m6","bolt"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "createTestItem: %s", w.Body.String())
	var item model.ItemResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&item), "createTestItem: decode")
	return item
}

func TestHandleCreateItem(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"name":"M6 bolt","container_id":1,"description":"DIN 933","tags":["m6","bolt"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var item model.ItemResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&item))

	assert.Positive(t, item.ID)
	assert.Equal(t, "M6 bolt", item.Name)
	require.NotNil(t, item.Description)
	assert.Equal(t, "DIN 933", *item.Description)
	assert.Len(t, item.Tags, 2)
	assert.NotEmpty(t, item.ContainerLabel)
	assert.NotEmpty(t, item.CreatedAt)
	assert.NotEmpty(t, item.UpdatedAt)
}

func TestHandleCreateItemValidation(t *testing.T) {
	router, _ := setupTestRouter(t)

	tests := []struct {
		name      string
		body      string
		wantError string
	}{
		{
			name:      "empty name",
			body:      `{"name":"","container_id":1,"tags":["m6"]}`,
			wantError: "name is required",
		},
		{
			name:      "name too long",
			body:      fmt.Sprintf(`{"name":"%s","container_id":1,"tags":["m6"]}`, strings.Repeat("x", 201)),
			wantError: "200 characters",
		},
		{
			name:      "no tags",
			body:      `{"name":"bolt","container_id":1,"tags":[]}`,
			wantError: "at least one tag",
		},
		{
			name: "too many tags",
			body: fmt.Sprintf(`{"name":"bolt","container_id":1,"tags":[%s]}`, func() string {
				parts := make([]string, 21)
				for i := range parts {
					parts[i] = fmt.Sprintf(`"t%d"`, i)
				}
				return strings.Join(parts, ",")
			}()),
			wantError: "at most 20 tags",
		},
		{
			name:      "empty tag",
			body:      `{"name":"bolt","container_id":1,"tags":[""]}`,
			wantError: "tag must not be empty",
		},
		{
			name:      "tag too long",
			body:      fmt.Sprintf(`{"name":"bolt","container_id":1,"tags":["%s"]}`, strings.Repeat("a", 51)),
			wantError: "50 characters",
		},
		{
			name:      "missing container_id",
			body:      `{"name":"bolt","tags":["m6"]}`,
			wantError: "container_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())

			var resp map[string]string
			require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
			assert.Contains(t, resp["error"], tc.wantError)
		})
	}
}

func TestHandleCreateItemContainerNotFound(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"name":"bolt","container_id":99999,"tags":["m6"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())
}

func TestHandleGetItem(t *testing.T) {
	router, _ := setupTestRouter(t)
	created := createTestItem(t, router)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items/%d", created.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var item model.ItemResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&item))
	assert.Equal(t, created.ID, item.ID)
	assert.Equal(t, created.Name, item.Name)
}

func TestHandleGetItemNotFound(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/items/99999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "item not found", resp["error"])
}

func TestHandleUpdateItem(t *testing.T) {
	router, _ := setupTestRouter(t)
	created := createTestItem(t, router)

	body := `{"name":"Updated","container_id":1}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/items/%d", created.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var item model.ItemResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&item))
	assert.Equal(t, "Updated", item.Name)
}

func TestHandleDeleteItem(t *testing.T) {
	router, _ := setupTestRouter(t)
	created := createTestItem(t, router)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/items/%d", created.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code, "body: %s", w.Body.String())

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items/%d", created.ID), nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleDeleteItemNotFound(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/items/99999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleAddTag(t *testing.T) {
	router, _ := setupTestRouter(t)
	created := createTestItem(t, router)

	body := `{"name":"stainless"}`
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/items/%d/tags", created.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var item model.ItemResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&item))
	assert.Contains(t, item.Tags, "stainless")
	assert.GreaterOrEqual(t, len(item.Tags), 3)
}

func TestHandleRemoveTag(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"name":"Remove tag test","container_id":1,"tags":["m6","bolt"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "create: %s", w.Body.String())
	var created model.ItemResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&created))

	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/items/%d/tags/m6", created.ID), nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code, "remove: %s", w.Body.String())

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items/%d", created.ID), nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var item model.ItemResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&item))
	assert.Equal(t, []string{"bolt"}, item.Tags)
}

func TestHandleListContainerItems(t *testing.T) {
	router, _ := setupTestRouter(t)

	for i := range 2 {
		body := fmt.Sprintf(`{"name":"List item %d","container_id":1,"tags":["tag%d"]}`, i, i)
		req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code, "create item %d: %s", i, w.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/containers/1/items", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var result model.ContainerWithItems
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Len(t, result.Items, 2)
	assert.NotEmpty(t, result.Label)
}

func TestHandleListItems(t *testing.T) {
	router, _ := setupTestRouter(t)

	for i := range 2 {
		body := fmt.Sprintf(`{"name":"All item %d","container_id":1,"tags":["tag%d"]}`, i, i)
		req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var items []model.ItemResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&items))
	assert.GreaterOrEqual(t, len(items), 2)
}

func TestHandleListTags(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"name":"Tags list","container_id":1,"tags":["alpha","beta"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var tags []model.TagResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&tags))
	assert.GreaterOrEqual(t, len(tags), 2)
}

func TestHandleListTagsPrefix(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"name":"Prefix test","container_id":1,"tags":["m6","m8","bolt"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/tags?q=m", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var tags []model.TagResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&tags))
	for _, tag := range tags {
		assert.True(t, strings.HasPrefix(tag.Name, "m"), "tag %q does not start with 'm'", tag.Name)
	}
}

func TestHandleCreateItemTagNormalization(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"name":"Normalize test","container_id":1,"tags":["M6"," Bolt "]}`
	req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	var item model.ItemResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&item))
	assert.Contains(t, item.Tags, "m6")
	assert.Contains(t, item.Tags, "bolt")
}

// --- Search handler tests ---

func TestHandleSearchByName(t *testing.T) {
	router, _ := setupTestRouter(t)
	createTestItem(t, router)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=bolt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp model.SearchResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "Test Bolt", resp.Results[0].Name)
}

func TestHandleSearchByTag(t *testing.T) {
	router, _ := setupTestRouter(t)
	createTestItem(t, router)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=m6", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp model.SearchResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp.Results, 1)
}

func TestHandleSearchEmpty(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp model.SearchResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Empty(t, resp.Results)
}

func TestHandleSearchMissingParam(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp model.SearchResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Empty(t, resp.Results)
}

func TestHandleSearchResponseShape(t *testing.T) {
	router, _ := setupTestRouter(t)
	createTestItem(t, router)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=bolt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var raw map[string]json.RawMessage
	require.NoError(t, json.NewDecoder(w.Body).Decode(&raw))
	require.Contains(t, raw, "results")

	var results []model.ItemResponse
	require.NoError(t, json.Unmarshal(raw["results"], &results))
	require.NotEmpty(t, results)

	r := results[0]
	assert.Positive(t, r.ID)
	assert.NotEmpty(t, r.Name)
	assert.NotEmpty(t, r.ContainerLabel)
	assert.NotNil(t, r.Tags)
}

func TestHandleSearchCaseInsensitive(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"name":"BOLT","container_id":1,"tags":["steel"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "create: %s", w.Body.String())

	req = httptest.NewRequest(http.MethodGet, "/api/search?q=bolt", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.SearchResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "BOLT", resp.Results[0].Name)
}

func TestHandleCreateItemDescriptionOptional(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"name":"Nut","container_id":1,"tags":["m6"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	var item model.ItemResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&item))
	assert.Nil(t, item.Description)
}

func TestErrorResponseFormat(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/items/99999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp, "error")
	assert.NotEmpty(t, resp["error"])
}

// --- Resize handler tests ---

func TestHandleResizeShelf_Success(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"rows":8,"cols":12}`
	req := httptest.NewRequest(http.MethodPut, "/api/shelf/resize", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var result map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.InDelta(t, float64(8), result["rows"], 0)
	assert.InDelta(t, float64(12), result["cols"], 0)
}

func TestHandleResizeShelf_Conflict(t *testing.T) {
	router, s := setupTestRouter(t)

	containerID, err := s.GetContainerIDByPosition(10, 5)
	require.NoError(t, err, "get container")

	body := fmt.Sprintf(`{"name":"Test Bolt","container_id":%d,"tags":["test"]}`, containerID)
	req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "create item: %s", w.Body.String())

	body2 := `{"rows":3,"cols":3}`
	req = httptest.NewRequest(http.MethodPut, "/api/shelf/resize", strings.NewReader(body2))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())

	var result map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Equal(t, true, result["blocked"])
	affected, ok := result["affected"].([]any)
	require.True(t, ok, "affected missing or wrong type: %v", result["affected"])
	assert.NotEmpty(t, affected)
}

func TestHandleResizeShelf_BadRequest_InvalidJSON(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodPut, "/api/shelf/resize", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
}

func TestValidateResize_TooSmall(t *testing.T) {
	req := &model.ResizeRequest{Rows: 0, Cols: 5}
	assert.Equal(t, "rows must be between 1 and 26", ValidateResizeRequest(req))
}

func TestValidateResize_TooLarge(t *testing.T) {
	req := &model.ResizeRequest{Rows: 5, Cols: 31}
	assert.Equal(t, "cols must be between 1 and 30", ValidateResizeRequest(req))
}

func TestValidateResize_Valid(t *testing.T) {
	req := &model.ResizeRequest{Rows: 5, Cols: 10}
	assert.Empty(t, ValidateResizeRequest(req))
}

func TestHandleResizeShelf_NameUpdate(t *testing.T) {
	router, s := setupTestRouter(t)

	body := `{"rows":5,"cols":10,"name":"Workshop"}`
	req := httptest.NewRequest(http.MethodPut, "/api/shelf/resize", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	name, err := s.GetShelfName()
	require.NoError(t, err, "query shelf name")
	assert.Equal(t, "Workshop", name)
}

func TestHandleResizeShelf_ValidationErrors(t *testing.T) {
	router, _ := setupTestRouter(t)

	tests := []struct {
		name string
		body string
	}{
		{"rows too small", `{"rows":0,"cols":5}`},
		{"rows too large", `{"rows":27,"cols":5}`},
		{"cols too small", `{"rows":5,"cols":0}`},
		{"cols too large", `{"rows":5,"cols":31}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/shelf/resize", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// --- Grid template handler ---

func TestHandleGrid(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	body := w.Body.String()
	assert.Contains(t, body, "grid-container")
	assert.Contains(t, body, "1A")
}

func TestHandleStaticFiles(t *testing.T) {
	router, _ := setupTestRouter(t)

	for _, path := range []string{"/static/css/app.css", "/static/css/grid.css", "/static/js/grid.js"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "GET %s", path)
	}
}

// --- Update/Delete validation edge cases ---

func TestHandleUpdateItemInvalidID(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodPut, "/api/items/abc", strings.NewReader(`{"name":"x","container_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleUpdateItemInvalidJSON(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodPut, "/api/items/1", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleUpdateItemValidation(t *testing.T) {
	router, _ := setupTestRouter(t)

	tests := []struct {
		name string
		body string
	}{
		{"empty name", `{"name":"","container_id":1}`},
		{"name too long", fmt.Sprintf(`{"name":"%s","container_id":1}`, strings.Repeat("x", 201))},
		{"missing container_id", `{"name":"bolt"}`},
		{"description too long", fmt.Sprintf(`{"name":"bolt","container_id":1,"description":"%s"}`, strings.Repeat("x", 1001))},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/items/1", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
		})
	}
}

func TestHandleUpdateItemNotFound(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"name":"x","container_id":1}`
	req := httptest.NewRequest(http.MethodPut, "/api/items/99999", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleUpdateItemContainerNotFound(t *testing.T) {
	router, _ := setupTestRouter(t)
	created := createTestItem(t, router)

	body := `{"name":"x","container_id":99999}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/items/%d", created.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleDeleteItemInvalidID(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/items/abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleGetItemInvalidID(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/items/abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateItemInvalidJSON(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateItemDescriptionTooLong(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := fmt.Sprintf(`{"name":"bolt","container_id":1,"tags":["m6"],"description":"%s"}`, strings.Repeat("x", 1001))
	req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Add tag edge cases ---

func TestHandleAddTagInvalidID(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/items/abc/tags", strings.NewReader(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleAddTagInvalidJSON(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/items/1/tags", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleAddTagEmpty(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/items/1/tags", strings.NewReader(`{"name":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleAddTagTooLong(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := fmt.Sprintf(`{"name":"%s"}`, strings.Repeat("a", 51))
	req := httptest.NewRequest(http.MethodPost, "/api/items/1/tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleAddTagItemNotFound(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/items/99999/tags", strings.NewReader(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- Remove tag edge cases ---

func TestHandleRemoveTagInvalidID(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/items/abc/tags/m6", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleRemoveTagNotFound(t *testing.T) {
	router, _ := setupTestRouter(t)
	created := createTestItem(t, router)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/items/%d/tags/nonexistent", created.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- Container items edge cases ---

func TestHandleListContainerItemsInvalidID(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/containers/abc/items", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleListContainerItemsNotFound(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/containers/99999/items", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- Resize name too long ---

func TestValidateResize_NameTooLong(t *testing.T) {
	longName := strings.Repeat("x", 101)
	req := &model.ResizeRequest{Rows: 5, Cols: 10, Name: &longName}
	assert.Contains(t, ValidateResizeRequest(req), "name must be at most 100 characters")
}

// ==========================================================================
// Mock store for error path testing
// ==========================================================================

type mockStore struct {
	pingFn                 func(ctx context.Context) error
	getGridDataFn          func() (*model.GridData, error)
	createItemFn           func(ctx context.Context, containerID int64, name string, description *string, tags []string) (*model.ItemResponse, error)
	getItemFn              func(ctx context.Context, id int64) (*model.ItemResponse, error)
	updateItemFn           func(ctx context.Context, id int64, name string, description *string, containerID int64) (*model.ItemResponse, error)
	deleteItemFn           func(ctx context.Context, id int64) error
	addTagToItemFn         func(ctx context.Context, itemID int64, tagName string) (*model.ItemResponse, error)
	removeTagFromItemFn    func(ctx context.Context, itemID int64, tagName string) error
	listItemsByContainerFn func(ctx context.Context, containerID int64) (*model.ContainerWithItems, error)
	listAllItemsFn         func(ctx context.Context) ([]model.ItemResponse, error)
	searchItemsFn          func(ctx context.Context, query string) ([]model.ItemResponse, error)
	searchItemsBatchFn     func(ctx context.Context, query string, tags []string) (*model.SearchResponse, error)
	listTagsFn             func(ctx context.Context, prefix string) ([]model.TagResponse, error)
	resizeShelfFn          func(ctx context.Context, newRows, newCols int) (*model.ResizeResult, error)
	updateShelfNameFn      func(ctx context.Context, name string) error
	getAuthSettingsFn      func(ctx context.Context) (*model.AuthSettings, error)
	updateAuthSettingsFn   func(ctx context.Context, settings *model.AuthSettings) error
	validateCredentialsFn  func(ctx context.Context, username, password string) (bool, error)
	getOIDCConfigFn        func(ctx context.Context) (*model.OIDCConfig, error)
	getOIDCConfigMaskedFn  func(ctx context.Context) (*model.OIDCConfig, error)
	saveOIDCConfigFn       func(ctx context.Context, cfg *model.OIDCConfig) error
	upsertOIDCUserFn       func(ctx context.Context, user *model.OIDCUser) (*model.OIDCUser, error)
	getOIDCUserBySubFn     func(ctx context.Context, sub, issuer string) (*model.OIDCUser, error)
	getOrCreateEncKeyFn    func(ctx context.Context) ([]byte, error)
	renameTagFn            func(ctx context.Context, tagID int64, newName string) error
	mergeTagsFn            func(ctx context.Context, sourceID, targetID int64) error
	deleteUnusedTagFn      func(ctx context.Context, tagID int64) error
	exportAllDataFn        func(ctx context.Context) (*model.ExportData, error)
	importAllDataFn        func(ctx context.Context, data *model.ExportData) error
}

func (m *mockStore) Ping(ctx context.Context) error {
	if m.pingFn != nil {
		return m.pingFn(ctx)
	}
	return nil
}

func (m *mockStore) GetGridData() (*model.GridData, error) { return m.getGridDataFn() }
func (m *mockStore) CreateItem(ctx context.Context, containerID int64, name string, description *string, tags []string) (*model.ItemResponse, error) {
	return m.createItemFn(ctx, containerID, name, description, tags)
}

func (m *mockStore) GetItem(ctx context.Context, id int64) (*model.ItemResponse, error) {
	return m.getItemFn(ctx, id)
}

func (m *mockStore) UpdateItem(ctx context.Context, id int64, name string, description *string, containerID int64) (*model.ItemResponse, error) {
	return m.updateItemFn(ctx, id, name, description, containerID)
}

func (m *mockStore) DeleteItem(ctx context.Context, id int64) error {
	return m.deleteItemFn(ctx, id)
}

func (m *mockStore) AddTagToItem(ctx context.Context, itemID int64, tagName string) (*model.ItemResponse, error) {
	return m.addTagToItemFn(ctx, itemID, tagName)
}

func (m *mockStore) RemoveTagFromItem(ctx context.Context, itemID int64, tagName string) error {
	return m.removeTagFromItemFn(ctx, itemID, tagName)
}

func (m *mockStore) ListItemsByContainer(ctx context.Context, containerID int64) (*model.ContainerWithItems, error) {
	return m.listItemsByContainerFn(ctx, containerID)
}

func (m *mockStore) ListAllItems(ctx context.Context) ([]model.ItemResponse, error) {
	return m.listAllItemsFn(ctx)
}

func (m *mockStore) SearchItems(ctx context.Context, query string) ([]model.ItemResponse, error) {
	return m.searchItemsFn(ctx, query)
}

func (m *mockStore) SearchItemsByTags(ctx context.Context, query string, _ []string) ([]model.ItemResponse, error) {
	return m.searchItemsFn(ctx, query)
}

func (m *mockStore) SearchItemsBatch(_ context.Context, _ string, _ []string) (*model.SearchResponse, error) {
	if m.searchItemsBatchFn != nil {
		return m.searchItemsBatchFn(context.Background(), "", nil)
	}
	if m.searchItemsFn != nil {
		// Fallback for error path testing
		_, err := m.searchItemsFn(context.Background(), "")
		if err != nil {
			return nil, err
		}
	}
	return &model.SearchResponse{Results: []model.SearchResult{}, TotalCount: 0}, nil
}

func (m *mockStore) ListTags(ctx context.Context, prefix string) ([]model.TagResponse, error) {
	return m.listTagsFn(ctx, prefix)
}

func (m *mockStore) ResizeShelf(ctx context.Context, newRows, newCols int) (*model.ResizeResult, error) {
	return m.resizeShelfFn(ctx, newRows, newCols)
}

func (m *mockStore) UpdateShelfName(ctx context.Context, name string) error {
	return m.updateShelfNameFn(ctx, name)
}

func (m *mockStore) GetAuthSettings(ctx context.Context) (*model.AuthSettings, error) {
	return m.getAuthSettingsFn(ctx)
}

func (m *mockStore) UpdateAuthSettings(ctx context.Context, settings *model.AuthSettings) error {
	return m.updateAuthSettingsFn(ctx, settings)
}

func (m *mockStore) ValidateCredentials(ctx context.Context, username, password string) (bool, error) {
	return m.validateCredentialsFn(ctx, username, password)
}

func (m *mockStore) GetOIDCConfig(ctx context.Context) (*model.OIDCConfig, error) {
	if m.getOIDCConfigFn != nil {
		return m.getOIDCConfigFn(ctx)
	}
	return nil, fmt.Errorf("not configured")
}

func (m *mockStore) GetOIDCConfigMasked(ctx context.Context) (*model.OIDCConfig, error) {
	if m.getOIDCConfigMaskedFn != nil {
		return m.getOIDCConfigMaskedFn(ctx)
	}
	return nil, fmt.Errorf("not configured")
}

func (m *mockStore) SaveOIDCConfig(ctx context.Context, cfg *model.OIDCConfig) error {
	if m.saveOIDCConfigFn != nil {
		return m.saveOIDCConfigFn(ctx, cfg)
	}
	return nil
}

func (m *mockStore) UpsertOIDCUser(ctx context.Context, user *model.OIDCUser) (*model.OIDCUser, error) {
	if m.upsertOIDCUserFn != nil {
		return m.upsertOIDCUserFn(ctx, user)
	}
	return user, nil
}

func (m *mockStore) GetOIDCUserBySub(ctx context.Context, sub, issuer string) (*model.OIDCUser, error) {
	if m.getOIDCUserBySubFn != nil {
		return m.getOIDCUserBySubFn(ctx, sub, issuer)
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockStore) GetOrCreateEncryptionKey(ctx context.Context) ([]byte, error) {
	if m.getOrCreateEncKeyFn != nil {
		return m.getOrCreateEncKeyFn(ctx)
	}
	return make([]byte, 32), nil
}

func (m *mockStore) ExportAllData(ctx context.Context) (*model.ExportData, error) {
	if m.exportAllDataFn != nil {
		return m.exportAllDataFn(ctx)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockStore) ImportAllData(ctx context.Context, data *model.ExportData) error {
	if m.importAllDataFn != nil {
		return m.importAllDataFn(ctx, data)
	}
	return fmt.Errorf("not implemented")
}

func (m *mockStore) FindDuplicates(_ context.Context) ([]model.DuplicateGroup, error) {
	return nil, nil
}

func (m *mockStore) RenameTag(ctx context.Context, tagID int64, newName string) error {
	if m.renameTagFn != nil {
		return m.renameTagFn(ctx, tagID, newName)
	}
	return nil
}

func (m *mockStore) MergeTags(ctx context.Context, sourceID, targetID int64) error {
	if m.mergeTagsFn != nil {
		return m.mergeTagsFn(ctx, sourceID, targetID)
	}
	return nil
}

func (m *mockStore) DeleteUnusedTag(ctx context.Context, tagID int64) error {
	if m.deleteUnusedTagFn != nil {
		return m.deleteUnusedTagFn(ctx, tagID)
	}
	return nil
}

func errStore() *mockStore {
	dbErr := fmt.Errorf("database error")
	return &mockStore{
		getGridDataFn: func() (*model.GridData, error) { return nil, dbErr },
		createItemFn: func(_ context.Context, _ int64, _ string, _ *string, _ []string) (*model.ItemResponse, error) {
			return nil, dbErr
		},
		getItemFn: func(_ context.Context, _ int64) (*model.ItemResponse, error) { return nil, dbErr },
		updateItemFn: func(_ context.Context, _ int64, _ string, _ *string, _ int64) (*model.ItemResponse, error) {
			return nil, dbErr
		},
		deleteItemFn:           func(_ context.Context, _ int64) error { return dbErr },
		addTagToItemFn:         func(_ context.Context, _ int64, _ string) (*model.ItemResponse, error) { return nil, dbErr },
		removeTagFromItemFn:    func(_ context.Context, _ int64, _ string) error { return dbErr },
		listItemsByContainerFn: func(_ context.Context, _ int64) (*model.ContainerWithItems, error) { return nil, dbErr },
		listAllItemsFn:         func(_ context.Context) ([]model.ItemResponse, error) { return nil, dbErr },
		searchItemsFn:          func(_ context.Context, _ string) ([]model.ItemResponse, error) { return nil, dbErr },
		listTagsFn:             func(_ context.Context, _ string) ([]model.TagResponse, error) { return nil, dbErr },
		resizeShelfFn:          func(_ context.Context, _, _ int) (*model.ResizeResult, error) { return nil, dbErr },
		updateShelfNameFn:      func(_ context.Context, _ string) error { return dbErr },
		getAuthSettingsFn:      func(_ context.Context) (*model.AuthSettings, error) { return nil, dbErr },
		updateAuthSettingsFn:   func(_ context.Context, _ *model.AuthSettings) error { return dbErr },
		validateCredentialsFn:  func(_ context.Context, _, _ string) (bool, error) { return false, dbErr },
	}
}

// errRouter creates a router backed by errStore and an in-memory session manager.
func errRouter() http.Handler {
	ms := session.NewMemoryStore(1*time.Hour, 10*time.Minute)
	mgr := session.NewManager(ms, 1*time.Hour, "Memory")
	srv := NewServer(errStore(), mgr, "test")
	return srv.Router()
}

// ==========================================================================
// Error path tests (internal server errors via mock)
// ==========================================================================

func TestHandleGridError(t *testing.T) {
	router := errRouter()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	// handleGrid renders error template, still 200
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Cannot load shelf")
}

func TestHandleCreateItemStoreError(t *testing.T) {
	router := errRouter()
	body := `{"name":"bolt","container_id":1,"tags":["m6"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleGetItemStoreError(t *testing.T) {
	router := errRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/items/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleUpdateItemStoreError(t *testing.T) {
	router := errRouter()
	body := `{"name":"bolt","container_id":1}`
	req := httptest.NewRequest(http.MethodPut, "/api/items/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleDeleteItemStoreError(t *testing.T) {
	router := errRouter()
	req := httptest.NewRequest(http.MethodDelete, "/api/items/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleAddTagStoreError(t *testing.T) {
	router := errRouter()
	body := `{"name":"m6"}`
	req := httptest.NewRequest(http.MethodPost, "/api/items/1/tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleRemoveTagStoreError(t *testing.T) {
	router := errRouter()
	req := httptest.NewRequest(http.MethodDelete, "/api/items/1/tags/m6", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleListContainerItemsStoreError(t *testing.T) {
	router := errRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/containers/1/items", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleListItemsStoreError(t *testing.T) {
	router := errRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleSearchStoreError(t *testing.T) {
	router := errRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=bolt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleListTagsStoreError(t *testing.T) {
	router := errRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleResizeShelfStoreError(t *testing.T) {
	router := errRouter()
	body := `{"rows":5,"cols":10}`
	req := httptest.NewRequest(http.MethodPut, "/api/shelf/resize", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Auth settings handler tests ---

func TestHandleGetAuthSettings(t *testing.T) {
	router, _ := setupTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/shelf/auth", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var settings model.AuthSettings
	require.NoError(t, json.NewDecoder(w.Body).Decode(&settings))
	assert.False(t, settings.Enabled)
	assert.Empty(t, settings.Username)
}

func TestHandleUpdateAuthSettings(t *testing.T) {
	router, s := setupTestRouter(t)

	body := `{"enabled":true,"username":"admin","password":"Secret123!@#"}`
	req := httptest.NewRequest(http.MethodPut, "/api/shelf/auth", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify persisted
	enabled, user, pass, err := s.GetRawAuthRow()
	require.NoError(t, err)
	assert.Equal(t, 1, enabled)
	assert.Equal(t, "admin", user)
	assert.True(t, strings.HasPrefix(pass, "$2a$"), "password should be bcrypt hashed")
}

func TestHandleUpdateAuthSettingsDisable(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"enabled":false,"username":"","password":""}`
	req := httptest.NewRequest(http.MethodPut, "/api/shelf/auth", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleUpdateAuthSettingsValidation(t *testing.T) {
	router, _ := setupTestRouter(t)

	tests := []struct {
		name string
		body string
	}{
		{"no username", `{"enabled":true,"username":"","password":"pass"}`},
		{"no password", `{"enabled":true,"username":"admin","password":""}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/shelf/auth", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestHandleUpdateAuthSettingsInvalidJSON(t *testing.T) {
	router, _ := setupTestRouter(t)
	req := httptest.NewRequest(http.MethodPut, "/api/shelf/auth", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleGetAuthSettingsStoreError(t *testing.T) {
	router := errRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/shelf/auth", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleUpdateAuthSettingsStoreError(t *testing.T) {
	router := errRouter()
	body := `{"enabled":true,"username":"admin","password":"Secret123!@#"}`
	req := httptest.NewRequest(http.MethodPut, "/api/shelf/auth", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- SearchItemsBatch handler tests ---

func TestSearchMatchedOn(t *testing.T) {
	router, s := setupTestRouter(t)
	ctx := context.Background()

	cid, err := s.GetContainerIDByPosition(1, 1)
	require.NoError(t, err)
	_, err = s.CreateItem(ctx, cid, "M6 Bolt", nil, []string{"m6", "bolt"})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=bolt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.SearchResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp.Results, 1)
	assert.Contains(t, resp.Results[0].MatchedOn, "name")
	assert.Contains(t, resp.Results[0].MatchedOn, "tag")
}

func TestSearchTotalCount(t *testing.T) {
	router, s := setupTestRouter(t)
	ctx := context.Background()

	cid, err := s.GetContainerIDByPosition(1, 1)
	require.NoError(t, err)
	for i := range 3 {
		_, err := s.CreateItem(ctx, cid, fmt.Sprintf("Bolt %d", i), nil, []string{"m6"})
		require.NoError(t, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=bolt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.SearchResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, 3, resp.TotalCount)
	assert.Len(t, resp.Results, 3)
}

func TestSearchEmptyParams(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.SearchResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Empty(t, resp.Results)
	assert.Equal(t, 0, resp.TotalCount)
}

func TestSearchTagsParam(t *testing.T) {
	router, s := setupTestRouter(t)
	ctx := context.Background()

	cid, err := s.GetContainerIDByPosition(1, 1)
	require.NoError(t, err)
	_, err = s.CreateItem(ctx, cid, "Socket Head", nil, []string{"m6", "din912"})
	require.NoError(t, err)
	_, err = s.CreateItem(ctx, cid, "Hex Head", nil, []string{"m6", "din933"})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/search?tags=m6,din912", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.SearchResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "Socket Head", resp.Results[0].Name)
}

func TestHandleSettingsPage(t *testing.T) {
	router, _ := setupTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "settings-layout")
	assert.Contains(t, body, "settings-sidebar")
	assert.Contains(t, body, "#shelf")
	assert.Contains(t, body, "#auth")
	assert.Contains(t, body, "#sessions")
	assert.Contains(t, body, "nav-badge")
	assert.Contains(t, body, `aria-label="Back to Grid"`)
}

func TestHandleSettingsShelfData(t *testing.T) {
	router, s := setupTestRouter(t)
	require.NoError(t, s.UpdateShelfName(context.Background(), "Test Shelf"))
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Test Shelf")
}

func TestGridPageHasSettingsLink(t *testing.T) {
	router, _ := setupTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `href="/settings"`, "grid page should have Settings link")
	assert.Contains(t, body, `aria-label="Settings"`, "Settings link should have aria-label")
}

func TestSettingsPageNavigation(t *testing.T) {
	router, _ := setupTestRouter(t)
	// Verify settings page has Back to Grid link
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `href="/"`, "settings page should have Back to Grid link")
	assert.Contains(t, body, "Back to Grid", "settings page should have Back to Grid text")
}

// ==========================================================================
// OIDC Handler Tests
// ==========================================================================

func newTestServerWithMock(t *testing.T, store StoreService) *Server {
	t.Helper()
	memStore := session.NewMemoryStore(1*time.Hour, 10*time.Minute)
	t.Cleanup(func() { memStore.Close() })
	mgr := session.NewManager(memStore, 1*time.Hour, "Memory")
	return NewServer(store, mgr, "test")
}

func oidcEnabledStore() *mockStore {
	return &mockStore{
		getAuthSettingsFn: func(_ context.Context) (*model.AuthSettings, error) {
			return &model.AuthSettings{Enabled: true, Username: "admin", HasPassword: true}, nil
		},
		getOIDCConfigFn: func(_ context.Context) (*model.OIDCConfig, error) {
			return &model.OIDCConfig{
				Enabled:     true,
				IssuerURL:   "https://idp.example.com",
				ClientID:    "test-client",
				DisplayName: "TestProvider",
			}, nil
		},
		getOrCreateEncKeyFn: func(_ context.Context) ([]byte, error) {
			return []byte("01234567890123456789012345678901"), nil
		},
	}
}

func oidcDisabledStore() *mockStore {
	return &mockStore{
		getAuthSettingsFn: func(_ context.Context) (*model.AuthSettings, error) {
			return &model.AuthSettings{Enabled: true, Username: "admin", HasPassword: true}, nil
		},
		getOIDCConfigFn: func(_ context.Context) (*model.OIDCConfig, error) {
			return &model.OIDCConfig{Enabled: false}, nil
		},
	}
}

func TestLoginPageWithOIDC(t *testing.T) {
	srv := newTestServerWithMock(t, oidcEnabledStore())
	router := srv.Router()
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "sso-btn", "should render SSO button")
	assert.Contains(t, body, "Sign in with TestProvider", "should show provider display name")
}

func TestLoginPageWithoutOIDC(t *testing.T) {
	srv := newTestServerWithMock(t, oidcDisabledStore())
	router := srv.Router()
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.NotContains(t, body, "sso-btn", "should NOT render SSO button")
	assert.Contains(t, body, `action="/login"`, "should still have login form")
}

func TestLoginPageWithOIDCError(t *testing.T) {
	srv := newTestServerWithMock(t, oidcEnabledStore())
	router := srv.Router()
	req := httptest.NewRequest(http.MethodGet, "/login?error=sso_unavailable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "SSO provider is unreachable", "should show SSO error message")
}

func TestOIDCCallbackMissingCode(t *testing.T) {
	srv := newTestServerWithMock(t, oidcEnabledStore())
	router := srv.Router()
	req := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/login?error=auth_failed")
}

func TestOIDCCallbackMissingStateCookie(t *testing.T) {
	srv := newTestServerWithMock(t, oidcEnabledStore())
	router := srv.Router()
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=abc&state=xyz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/login?error=auth_failed")
}

func TestOIDCCallbackStateMismatch(t *testing.T) {
	testKey := []byte("01234567890123456789012345678901")
	ms := oidcEnabledStore()

	srv := newTestServerWithMock(t, ms)
	router := srv.Router()

	// Encrypt a state cookie with state="correct_state"
	encrypted, err := oidcpkg.EncryptStateCookie(testKey, &oidcpkg.StateCookie{
		State:    "correct_state",
		Nonce:    "test_nonce",
		Verifier: "test_verifier",
	})
	require.NoError(t, err)

	// Request with wrong state query param
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=abc&state=wrong_state", nil)
	req.AddCookie(&http.Cookie{
		Name:  oidcpkg.StateCookieName,
		Value: encrypted,
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/login?error=auth_failed")
}

func TestOIDCStartDisabled(t *testing.T) {
	srv := newTestServerWithMock(t, oidcDisabledStore())
	router := srv.Router()
	req := httptest.NewRequest(http.MethodGet, "/auth/oidc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))
}

// --- Tag management handler tests ---

// createTaggedItem creates an item with given tags via the API and returns the item.
func createTaggedItem(t *testing.T, router http.Handler, name string, containerID int64, tags []string) model.ItemResponse { //nolint:unparam // containerID kept explicit for test readability
	t.Helper()
	tagsJSON, err := json.Marshal(tags)
	require.NoError(t, err)
	body := fmt.Sprintf(`{"name":%q,"container_id":%d,"tags":%s}`, name, containerID, tagsJSON)
	req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "createTaggedItem: %s", w.Body.String())
	var item model.ItemResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&item))
	return item
}

// getTagIDByName fetches tags from the API and returns the ID of the tag with the given name.
func getTagIDByName(t *testing.T, router http.Handler, name string) int64 {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/tags?q="+name, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var tags []model.TagResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&tags))
	for _, tag := range tags {
		if tag.Name == name {
			return tag.ID
		}
	}
	t.Fatalf("tag %q not found", name)
	return 0
}

func TestHandleRenameTag(t *testing.T) {
	router, _ := setupTestRouter(t)
	createTaggedItem(t, router, "M6 Bolt", 1, []string{"bolt"})

	tagID := getTagIDByName(t, router, "bolt")

	body := `{"name":"screw"}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/tags/%d", tagID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, true, resp["ok"])
}

func TestHandleRenameTagConflict(t *testing.T) {
	router, _ := setupTestRouter(t)
	createTaggedItem(t, router, "M6 Bolt", 1, []string{"bolt"})
	createTaggedItem(t, router, "M6 Screw", 1, []string{"screw"})

	tagID := getTagIDByName(t, router, "bolt")

	body := `{"name":"screw"}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/tags/%d", tagID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, true, resp["merge_needed"])
	assert.NotNil(t, resp["target"])
	assert.NotNil(t, resp["source"])
}

func TestHandleRenameTagMerge(t *testing.T) {
	router, _ := setupTestRouter(t)
	createTaggedItem(t, router, "M6 Bolt", 1, []string{"bolt"})
	createTaggedItem(t, router, "M6 Screw", 1, []string{"screw"})

	tagID := getTagIDByName(t, router, "bolt")

	body := `{"name":"screw","force_merge":true}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/tags/%d", tagID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, true, resp["ok"])
}

func TestHandleRenameTagValidation(t *testing.T) {
	router, _ := setupTestRouter(t)
	createTaggedItem(t, router, "M6 Bolt", 1, []string{"bolt"})
	tagID := getTagIDByName(t, router, "bolt")

	tests := []struct {
		name string
		body string
	}{
		{"empty name", `{"name":""}`},
		{"name too long", fmt.Sprintf(`{"name":"%s"}`, strings.Repeat("x", 51))},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/tags/%d", tagID), strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestHandleDeleteTag(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Create an item with a tag, then delete the item to leave an orphan tag
	item := createTaggedItem(t, router, "Temp Bolt", 1, []string{"unused-tag"})
	tagID := getTagIDByName(t, router, "unused-tag")

	// Delete the item so the tag becomes unused
	delReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/items/%d", item.ID), nil)
	delW := httptest.NewRecorder()
	router.ServeHTTP(delW, delReq)
	require.Equal(t, http.StatusNoContent, delW.Code)

	// Now delete the unused tag
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/tags/%d", tagID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestHandleDeleteTagInUse(t *testing.T) {
	router, _ := setupTestRouter(t)
	createTaggedItem(t, router, "M6 Bolt", 1, []string{"bolt"})
	tagID := getTagIDByName(t, router, "bolt")

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/tags/%d", tagID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "in use")
}

func TestHandleDuplicates_WithDuplicates(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Insert two items with same name in different containers (1 and 2), same tags
	body1 := `{"name":"M4 bolt","container_id":1,"tags":["metric"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body1))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	body2 := `{"name":"m4 bolt","container_id":2,"tags":["metric"]}`
	req = httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body2))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	// GET /api/duplicates
	req = httptest.NewRequest(http.MethodGet, "/api/duplicates", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	// Capture raw body before decoding (Decode consumes the reader)
	raw := w.Body.String()

	var groups []model.DuplicateGroup
	require.NoError(t, json.Unmarshal([]byte(raw), &groups))
	require.Len(t, groups, 1)
	assert.Equal(t, 2, groups[0].Count)
	assert.Len(t, groups[0].Containers, 2)
	assert.Len(t, groups[0].Tags, 1)
	assert.Equal(t, "metric", groups[0].Tags[0])

	// Verify JSON uses "label" field name
	assert.Contains(t, raw, `"label"`)
	assert.NotContains(t, raw, `"container_label"`)
}

func TestHandleDuplicates_Empty(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/duplicates", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var groups []model.DuplicateGroup
	require.NoError(t, json.NewDecoder(w.Body).Decode(&groups))
	assert.Empty(t, groups)
}

func TestHandleDuplicates_StoreError(t *testing.T) {
	// For a store error test, we close the store to force an error
	s := &store.Store{}
	tmpFile := filepath.Join(t.TempDir(), "test.db")
	require.NoError(t, s.Open(tmpFile))

	memStore := session.NewMemoryStore(1*time.Hour, 10*time.Minute)
	t.Cleanup(func() { memStore.Close() })
	mgr := session.NewManager(memStore, 1*time.Hour, "Memory")

	srv := NewServer(s, mgr, "test")
	router := srv.Router()

	// Close store to force error
	s.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/duplicates", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Login POST tests ---

func TestHandleLoginPostSuccess(t *testing.T) {
	router, s := setupTestRouter(t)
	ctx := context.Background()

	err := s.UpdateAuthSettings(ctx, &model.AuthSettings{
		Enabled:  true,
		Username: "admin",
		Password: "secret123",
	})
	require.NoError(t, err)

	body := "username=admin&password=secret123"
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/", w.Header().Get("Location"))
}

func TestHandleLoginPostInvalidCredentials(t *testing.T) {
	router, s := setupTestRouter(t)
	ctx := context.Background()

	err := s.UpdateAuthSettings(ctx, &model.AuthSettings{
		Enabled:  true,
		Username: "admin",
		Password: "secret123",
	})
	require.NoError(t, err)

	body := "username=admin&password=wrong"
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid username or password")
}

// --- Logout test ---

func TestHandleLogout(t *testing.T) {
	router, s := setupTestRouter(t)
	ctx := context.Background()

	err := s.UpdateAuthSettings(ctx, &model.AuthSettings{
		Enabled:  true,
		Username: "admin",
		Password: "pass123",
	})
	require.NoError(t, err)

	// Login to get session cookie
	loginBody := "username=admin&password=pass123"
	loginReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginW := httptest.NewRecorder()
	router.ServeHTTP(loginW, loginReq)
	require.Equal(t, http.StatusFound, loginW.Code)

	cookies := loginW.Result().Cookies()

	logoutReq := httptest.NewRequest(http.MethodGet, "/logout", nil)
	for _, c := range cookies {
		logoutReq.AddCookie(c)
	}
	logoutW := httptest.NewRecorder()
	router.ServeHTTP(logoutW, logoutReq)

	assert.Equal(t, http.StatusFound, logoutW.Code)
	assert.Equal(t, "/login", logoutW.Header().Get("Location"))
}

// --- Export test ---

func TestHandleExport(t *testing.T) {
	router, s := setupTestRouter(t)
	ctx := context.Background()

	// Use container ID 1 (seeded by default shelf)
	_, err := s.CreateItem(ctx, 1, "Export Bolt", nil, []string{"m6", "steel"})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/export", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "screws-box-export-")

	var data model.ExportData
	require.NoError(t, json.NewDecoder(w.Body).Decode(&data))
	assert.Equal(t, 1, data.Version)
	assert.NotEmpty(t, data.Shelf.Name)
}

// --- Import validate + confirm tests ---

func TestHandleImportValidateAndConfirm(t *testing.T) {
	router, s := setupTestRouter(t)
	ctx := context.Background()

	exportData, err := s.ExportAllData(ctx)
	require.NoError(t, err)

	exportJSON, err := json.Marshal(exportData)
	require.NoError(t, err)

	boundary := "testboundary123"
	var body strings.Builder
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"export.json\"\r\n")
	body.WriteString("Content-Type: application/json\r\n\r\n")
	body.Write(exportJSON)
	body.WriteString("\r\n--" + boundary + "--\r\n")

	req := httptest.NewRequest(http.MethodPost, "/api/import/validate", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var validateResp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&validateResp))
	token, ok := validateResp["token"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, token)

	// Confirm step
	confirmBody := fmt.Sprintf(`{"token":"%s"}`, token)
	confirmReq := httptest.NewRequest(http.MethodPost, "/api/import/confirm", strings.NewReader(confirmBody))
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmW := httptest.NewRecorder()
	router.ServeHTTP(confirmW, confirmReq)

	assert.Equal(t, http.StatusOK, confirmW.Code)
	assert.Contains(t, confirmW.Body.String(), "Import complete")
}

func TestHandleImportValidateNoFile(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/import/validate", nil)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=empty")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "No file uploaded")
}

func TestHandleImportValidateInvalidJSON(t *testing.T) {
	router, _ := setupTestRouter(t)

	boundary := "testboundary456"
	var body strings.Builder
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"bad.json\"\r\n")
	body.WriteString("Content-Type: application/json\r\n\r\n")
	body.WriteString("not json at all")
	body.WriteString("\r\n--" + boundary + "--\r\n")

	req := httptest.NewRequest(http.MethodPost, "/api/import/validate", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "valid JSON")
}

func TestHandleImportValidateBadVersion(t *testing.T) {
	router, _ := setupTestRouter(t)

	data := `{"version":99,"exported_at":"2026-01-01T00:00:00Z","shelf":{"name":"Test","rows":1,"cols":1,"containers":[]}}`

	boundary := "testboundary789"
	var body strings.Builder
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"bad.json\"\r\n")
	body.WriteString("Content-Type: application/json\r\n\r\n")
	body.WriteString(data)
	body.WriteString("\r\n--" + boundary + "--\r\n")

	req := httptest.NewRequest(http.MethodPost, "/api/import/validate", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Unsupported file version")
}

func TestHandleImportConfirmExpiredToken(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"token":"nonexistent-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/import/confirm", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "expired")
}

func TestHandleImportConfirmMissingToken(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/import/confirm", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Missing import token")
}

// --- OIDC Config API tests ---

func TestHandleGetOIDCConfig(t *testing.T) {
	router, s := setupTestRouter(t)
	ctx := context.Background()

	err := s.SaveOIDCConfig(ctx, &model.OIDCConfig{
		Enabled:      true,
		IssuerURL:    "https://auth.example.com",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		DisplayName:  "Test SSO",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/oidc/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var cfg model.OIDCConfig
	require.NoError(t, json.NewDecoder(w.Body).Decode(&cfg))
	assert.Equal(t, "https://auth.example.com", cfg.IssuerURL)
	assert.Equal(t, "test-client", cfg.ClientID)
	assert.NotEqual(t, "test-secret", cfg.ClientSecret)
}

// ==========================================================================
// Additional coverage tests for low-coverage handlers
// ==========================================================================

// setupAuthEnabled creates a test router with auth enabled using a strong password.
func setupAuthEnabled(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	router, s := setupTestRouter(t)
	ctx := context.Background()
	err := s.UpdateAuthSettings(ctx, &model.AuthSettings{
		Enabled:  true,
		Username: "admin",
		Password: "Secret123!@#",
	})
	require.NoError(t, err)
	return router, s
}

// --- handleLoginPost additional coverage ---

func TestHandleLoginPost_EmptyFields(t *testing.T) {
	router, _ := setupAuthEnabled(t)

	body := "username=&password="
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "empty credentials should re-render login page")
	assert.Contains(t, w.Body.String(), "Invalid username or password")
}

func TestHandleLoginPost_StoreError(t *testing.T) {
	ms := &mockStore{
		getAuthSettingsFn: func(_ context.Context) (*model.AuthSettings, error) {
			return &model.AuthSettings{Enabled: true, Username: "admin", HasPassword: true}, nil
		},
		validateCredentialsFn: func(_ context.Context, _, _ string) (bool, error) {
			return false, fmt.Errorf("database error")
		},
		getOIDCConfigFn: func(_ context.Context) (*model.OIDCConfig, error) {
			return &model.OIDCConfig{Enabled: false}, nil
		},
	}
	srv := newTestServerWithMock(t, ms)
	router := srv.Router()

	body := "username=admin&password=Secret123!@#"
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "store error should re-render login page")
	assert.Contains(t, w.Body.String(), "Internal error")
}

func TestHandleLoginPost_SessionCookie(t *testing.T) {
	router, _ := setupAuthEnabled(t)

	body := "username=admin&password=Secret123!@#"
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusFound, w.Code)

	var hasSessionCookie bool
	for _, c := range w.Result().Cookies() {
		if c.Name == session.CookieName {
			hasSessionCookie = true
			assert.NotEmpty(t, c.Value)
		}
	}
	assert.True(t, hasSessionCookie, "should set session cookie on successful login")
}

// --- handleExport additional coverage ---

func TestHandleExport_ContentDisposition(t *testing.T) {
	router, _ := setupTestRouter(t)
	createTestItem(t, router)

	req := httptest.NewRequest(http.MethodGet, "/api/export", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "screws-box-export-")
	assert.Contains(t, w.Header().Get("Content-Disposition"), ".json")
}

func TestHandleExport_StructureWithItems(t *testing.T) {
	router, _ := setupTestRouter(t)
	createTestItem(t, router)

	req := httptest.NewRequest(http.MethodGet, "/api/export", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var data model.ExportData
	require.NoError(t, json.NewDecoder(w.Body).Decode(&data))
	assert.Equal(t, 1, data.Version)
	assert.NotEmpty(t, data.ExportedAt)
	assert.Positive(t, data.Shelf.Rows)
	assert.Positive(t, data.Shelf.Cols)

	// At least one container should have items
	itemCount := 0
	for _, c := range data.Shelf.Containers {
		itemCount += len(c.Items)
	}
	assert.Positive(t, itemCount, "export should include the created item")
}

func TestHandleExport_StoreError(t *testing.T) {
	ms := errStore()
	ms.exportAllDataFn = func(_ context.Context) (*model.ExportData, error) {
		return nil, fmt.Errorf("database error")
	}
	memStore := session.NewMemoryStore(1*time.Hour, 10*time.Minute)
	t.Cleanup(func() { memStore.Close() })
	mgr := session.NewManager(memStore, 1*time.Hour, "Memory")
	srv := NewServer(ms, mgr, "test")
	router := srv.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/export", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleExport_EmptyShelf(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/export", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var data model.ExportData
	require.NoError(t, json.NewDecoder(w.Body).Decode(&data))

	itemCount := 0
	for _, c := range data.Shelf.Containers {
		itemCount += len(c.Items)
	}
	assert.Equal(t, 0, itemCount, "empty shelf should export zero items")
}

// --- handleImportValidate additional coverage ---

func TestHandleImportValidate_MissingRequiredFields(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Version 0, empty shelf name, zero rows/cols
	data := `{"version":0,"shelf":{"name":"","rows":0,"cols":0}}`

	boundary := "testmissing123"
	var body strings.Builder
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"import.json\"\r\n")
	body.WriteString("Content-Type: application/json\r\n\r\n")
	body.WriteString(data)
	body.WriteString("\r\n--" + boundary + "--\r\n")

	req := httptest.NewRequest(http.MethodPost, "/api/import/validate", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	errs, ok := resp["errors"].([]any)
	require.True(t, ok)
	// Should report: version, name, rows, cols
	assert.GreaterOrEqual(t, len(errs), 4, "should report all validation errors")
}

func TestHandleImportValidate_SummaryCountsMultipleItems(t *testing.T) {
	router, _ := setupTestRouter(t)

	importData := model.ExportData{
		Version:    1,
		ExportedAt: "2026-01-01T00:00:00Z",
		Shelf: model.ExportShelf{
			Name: "Test",
			Rows: 2,
			Cols: 2,
			Containers: []model.ExportContainer{
				{
					Col: 1, Row: 1, Label: "1A",
					Items: []model.ExportItem{
						{Name: "Bolt", Tags: []string{"m6", "steel"}},
						{Name: "Nut", Tags: []string{"m6", "brass"}},
					},
				},
			},
		},
	}
	jsonBytes, err := json.Marshal(importData)
	require.NoError(t, err)

	boundary := "testcounts123"
	var body strings.Builder
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"import.json\"\r\n")
	body.WriteString("Content-Type: application/json\r\n\r\n")
	body.Write(jsonBytes)
	body.WriteString("\r\n--" + boundary + "--\r\n")

	req := httptest.NewRequest(http.MethodPost, "/api/import/validate", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	summary, ok := resp["summary"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, float64(2), summary["items"], 0, "should count 2 items")
	assert.InDelta(t, float64(3), summary["tags"], 0, "should count 3 unique tags (m6, steel, brass)")
}

// --- handleImportConfirm additional coverage ---

func TestHandleImportConfirm_InvalidJSON(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/import/confirm", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleImportConfirm_TokenUsedTwice(t *testing.T) {
	router, s := setupTestRouter(t)
	ctx := context.Background()

	exportData, err := s.ExportAllData(ctx)
	require.NoError(t, err)

	exportJSON, err := json.Marshal(exportData)
	require.NoError(t, err)

	boundary := "testtwice123"
	var body strings.Builder
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"export.json\"\r\n")
	body.WriteString("Content-Type: application/json\r\n\r\n")
	body.Write(exportJSON)
	body.WriteString("\r\n--" + boundary + "--\r\n")

	req := httptest.NewRequest(http.MethodPost, "/api/import/validate", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var validateResp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&validateResp))
	token := validateResp["token"].(string)

	// First confirm succeeds
	confirmBody := fmt.Sprintf(`{"token":"%s"}`, token)
	req = httptest.NewRequest(http.MethodPost, "/api/import/confirm", strings.NewReader(confirmBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Second confirm with same token fails
	req = httptest.NewRequest(http.MethodPost, "/api/import/confirm", strings.NewReader(confirmBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "expired")
}

// --- handleOIDCStart additional coverage ---

func TestOIDCStart_EnabledButProviderUnreachable(t *testing.T) {
	ms := oidcEnabledStore()
	srv := newTestServerWithMock(t, ms)
	router := srv.Router()

	req := httptest.NewRequest(http.MethodGet, "/auth/oidc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/login?error=sso_unavailable",
		"unreachable OIDC provider should redirect with sso_unavailable error")
}

func TestOIDCStart_EncryptionKeyError(t *testing.T) {
	ms := oidcEnabledStore()
	ms.getOrCreateEncKeyFn = func(_ context.Context) ([]byte, error) {
		return nil, fmt.Errorf("key error")
	}
	srv := newTestServerWithMock(t, ms)
	router := srv.Router()

	req := httptest.NewRequest(http.MethodGet, "/auth/oidc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/login?error=auth_failed")
}

func TestOIDCStart_ConfigError(t *testing.T) {
	ms := &mockStore{
		getAuthSettingsFn: func(_ context.Context) (*model.AuthSettings, error) {
			return &model.AuthSettings{Enabled: true}, nil
		},
		getOIDCConfigFn: func(_ context.Context) (*model.OIDCConfig, error) {
			return nil, fmt.Errorf("config error")
		},
	}
	srv := newTestServerWithMock(t, ms)
	router := srv.Router()

	req := httptest.NewRequest(http.MethodGet, "/auth/oidc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))
}

// --- handleUpdateOIDCConfig additional coverage ---

func TestHandleUpdateOIDCConfig_DisableOIDC(t *testing.T) {
	ms := &mockStore{
		getAuthSettingsFn: func(_ context.Context) (*model.AuthSettings, error) {
			return &model.AuthSettings{Enabled: false}, nil
		},
		getOIDCConfigFn: func(_ context.Context) (*model.OIDCConfig, error) {
			return &model.OIDCConfig{Enabled: true, IssuerURL: "https://idp.example.com", ClientID: "cid"}, nil
		},
		getOIDCConfigMaskedFn: func(_ context.Context) (*model.OIDCConfig, error) {
			return &model.OIDCConfig{Enabled: false}, nil
		},
		saveOIDCConfigFn: func(_ context.Context, _ *model.OIDCConfig) error {
			return nil
		},
	}
	srv := newTestServerWithMock(t, ms)
	router := srv.Router()

	body := `{"enabled":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/oidc/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
}

func TestHandleUpdateOIDCConfig_InvalidJSON(t *testing.T) {
	ms := &mockStore{
		getAuthSettingsFn: func(_ context.Context) (*model.AuthSettings, error) {
			return &model.AuthSettings{Enabled: false}, nil
		},
	}
	srv := newTestServerWithMock(t, ms)
	router := srv.Router()

	req := httptest.NewRequest(http.MethodPut, "/api/oidc/config", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleUpdateOIDCConfig_EnableMissingFields(t *testing.T) {
	ms := &mockStore{
		getAuthSettingsFn: func(_ context.Context) (*model.AuthSettings, error) {
			return &model.AuthSettings{Enabled: false}, nil
		},
		getOIDCConfigFn: func(_ context.Context) (*model.OIDCConfig, error) {
			return &model.OIDCConfig{Enabled: false}, nil
		},
	}
	srv := newTestServerWithMock(t, ms)
	router := srv.Router()

	body := `{"enabled":true,"issuer_url":"","client_id":"","client_secret":"sec"}`
	req := httptest.NewRequest(http.MethodPut, "/api/oidc/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "required")
}

func TestHandleUpdateOIDCConfig_EnableMissingSecret(t *testing.T) {
	ms := &mockStore{
		getAuthSettingsFn: func(_ context.Context) (*model.AuthSettings, error) {
			return &model.AuthSettings{Enabled: false}, nil
		},
		getOIDCConfigFn: func(_ context.Context) (*model.OIDCConfig, error) {
			return &model.OIDCConfig{Enabled: false, ClientSecret: ""}, nil
		},
	}
	srv := newTestServerWithMock(t, ms)
	router := srv.Router()

	body := `{"enabled":true,"issuer_url":"https://example.com","client_id":"cid","client_secret":""}`
	req := httptest.NewRequest(http.MethodPut, "/api/oidc/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "required")
}

func TestHandleUpdateOIDCConfig_SaveError(t *testing.T) {
	ms := &mockStore{
		getAuthSettingsFn: func(_ context.Context) (*model.AuthSettings, error) {
			return &model.AuthSettings{Enabled: false}, nil
		},
		getOIDCConfigFn: func(_ context.Context) (*model.OIDCConfig, error) {
			return &model.OIDCConfig{Enabled: false}, nil
		},
		saveOIDCConfigFn: func(_ context.Context, _ *model.OIDCConfig) error {
			return fmt.Errorf("save error")
		},
	}
	srv := newTestServerWithMock(t, ms)
	router := srv.Router()

	body := `{"enabled":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/oidc/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- handleLogout additional coverage ---

func TestHandleLogout_ClearsSession(t *testing.T) {
	router, _ := setupAuthEnabled(t)

	// Login to create a session
	loginBody := "username=admin&password=Secret123!@#"
	loginReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginW := httptest.NewRecorder()
	router.ServeHTTP(loginW, loginReq)
	require.Equal(t, http.StatusFound, loginW.Code)

	var sessionCookieValue string
	for _, c := range loginW.Result().Cookies() {
		if c.Name == session.CookieName {
			sessionCookieValue = c.Value
		}
	}
	require.NotEmpty(t, sessionCookieValue)

	// Logout with the session cookie
	logoutReq := httptest.NewRequest(http.MethodGet, "/logout", nil)
	logoutReq.AddCookie(&http.Cookie{Name: session.CookieName, Value: sessionCookieValue})
	logoutW := httptest.NewRecorder()
	router.ServeHTTP(logoutW, logoutReq)

	assert.Equal(t, http.StatusFound, logoutW.Code)
	assert.Equal(t, "/login", logoutW.Header().Get("Location"))

	// Session cookie should be cleared
	var sessionCleared bool
	for _, c := range logoutW.Result().Cookies() {
		if c.Name == session.CookieName && c.MaxAge < 0 {
			sessionCleared = true
		}
	}
	assert.True(t, sessionCleared, "logout should clear the session cookie")
}

func TestHandleLogout_WithoutSession(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))
}

// --- Login page (GET /login) additional coverage ---

func TestHandleLoginPage_AuthDisabled_Redirects(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/", w.Header().Get("Location"),
		"should redirect to / when auth is disabled")
}

func TestHandleLoginPage_AuthEnabled_ShowsForm(t *testing.T) {
	router, _ := setupAuthEnabled(t)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `action="/login"`)
}

func TestHandleLoginPage_AlreadyLoggedIn_Redirects(t *testing.T) {
	router, _ := setupAuthEnabled(t)

	// Login first
	loginReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("username=admin&password=Secret123!@#"))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginW := httptest.NewRecorder()
	router.ServeHTTP(loginW, loginReq)
	require.Equal(t, http.StatusFound, loginW.Code)

	var sessionCookie *http.Cookie
	for _, c := range loginW.Result().Cookies() {
		if c.Name == session.CookieName {
			sessionCookie = c
		}
	}
	require.NotNil(t, sessionCookie)

	// Visit login page with active session
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.AddCookie(sessionCookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/", w.Header().Get("Location"),
		"already logged in user should be redirected to /")
}

func TestHandleLoginPage_AuthFailedErrorParam(t *testing.T) {
	ms := oidcEnabledStore()
	srv := newTestServerWithMock(t, ms)
	router := srv.Router()

	req := httptest.NewRequest(http.MethodGet, "/login?error=auth_failed", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Authentication failed")
}

func TestHandleLoginPage_SSOUnavailableErrorParam(t *testing.T) {
	ms := oidcEnabledStore()
	srv := newTestServerWithMock(t, ms)
	router := srv.Router()

	req := httptest.NewRequest(http.MethodGet, "/login?error=sso_unavailable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "SSO provider is unreachable")
}

// Ensure oidcpkg import is referenced (used in existing tests above and below).
var _ = oidcpkg.StateCookieName
