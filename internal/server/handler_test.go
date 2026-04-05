package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"screws-box/internal/model"
	"screws-box/internal/store"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	s := &store.Store{}
	tmpFile := filepath.Join(t.TempDir(), "test.db")
	require.NoError(t, s.Open(tmpFile))
	t.Cleanup(func() { s.Close() })
	router := NewRouter(s)
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

	var resp map[string][]model.ItemResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp["results"], 1)
	assert.Equal(t, "Test Bolt", resp["results"][0].Name)
}

func TestHandleSearchByTag(t *testing.T) {
	router, _ := setupTestRouter(t)
	createTestItem(t, router)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=m6", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string][]model.ItemResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp["results"], 1)
}

func TestHandleSearchEmpty(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string][]model.ItemResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Empty(t, resp["results"])
}

func TestHandleSearchMissingParam(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string][]model.ItemResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Empty(t, resp["results"])
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

	var resp map[string][]model.ItemResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp["results"], 1)
	assert.Equal(t, "BOLT", resp["results"][0].Name)
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
	listTagsFn             func(ctx context.Context, prefix string) ([]model.TagResponse, error)
	resizeShelfFn          func(ctx context.Context, newRows, newCols int) (*model.ResizeResult, error)
	updateShelfNameFn      func(ctx context.Context, name string) error
	getAuthSettingsFn      func(ctx context.Context) (*model.AuthSettings, error)
	updateAuthSettingsFn   func(ctx context.Context, settings *model.AuthSettings) error
	validateCredentialsFn  func(ctx context.Context, username, password string) (bool, error)
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

// ==========================================================================
// Error path tests (internal server errors via mock)
// ==========================================================================

func TestHandleGridError(t *testing.T) {
	router := NewRouter(errStore())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	// handleGrid renders error template, still 200
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Cannot load shelf")
}

func TestHandleCreateItemStoreError(t *testing.T) {
	router := NewRouter(errStore())
	body := `{"name":"bolt","container_id":1,"tags":["m6"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleGetItemStoreError(t *testing.T) {
	router := NewRouter(errStore())
	req := httptest.NewRequest(http.MethodGet, "/api/items/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleUpdateItemStoreError(t *testing.T) {
	router := NewRouter(errStore())
	body := `{"name":"bolt","container_id":1}`
	req := httptest.NewRequest(http.MethodPut, "/api/items/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleDeleteItemStoreError(t *testing.T) {
	router := NewRouter(errStore())
	req := httptest.NewRequest(http.MethodDelete, "/api/items/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleAddTagStoreError(t *testing.T) {
	router := NewRouter(errStore())
	body := `{"name":"m6"}`
	req := httptest.NewRequest(http.MethodPost, "/api/items/1/tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleRemoveTagStoreError(t *testing.T) {
	router := NewRouter(errStore())
	req := httptest.NewRequest(http.MethodDelete, "/api/items/1/tags/m6", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleListContainerItemsStoreError(t *testing.T) {
	router := NewRouter(errStore())
	req := httptest.NewRequest(http.MethodGet, "/api/containers/1/items", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleListItemsStoreError(t *testing.T) {
	router := NewRouter(errStore())
	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleSearchStoreError(t *testing.T) {
	router := NewRouter(errStore())
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=bolt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleListTagsStoreError(t *testing.T) {
	router := NewRouter(errStore())
	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleResizeShelfStoreError(t *testing.T) {
	router := NewRouter(errStore())
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
	router := NewRouter(errStore())
	req := httptest.NewRequest(http.MethodGet, "/api/shelf/auth", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleUpdateAuthSettingsStoreError(t *testing.T) {
	router := NewRouter(errStore())
	body := `{"enabled":true,"username":"admin","password":"Secret123!@#"}`
	req := httptest.NewRequest(http.MethodPut, "/api/shelf/auth", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
