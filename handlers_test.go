package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// setupTestRouter creates a chi router with a real in-memory store for integration tests.
func setupTestRouter(t *testing.T) (http.Handler, *Store) {
	t.Helper()
	store := openTestStore(t) // reuse from store_test.go (same package)
	router := newRouter(store)
	return router, store
}

// createTestItem creates an item via POST /api/items and returns the response.
func createTestItem(t *testing.T, router http.Handler) ItemResponse {
	t.Helper()
	body := `{"name":"Test Bolt","container_id":1,"tags":["m6","bolt"]}`
	req := httptest.NewRequest("POST", "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("createTestItem: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var item ItemResponse
	if err := json.NewDecoder(w.Body).Decode(&item); err != nil {
		t.Fatalf("createTestItem: decode: %v", err)
	}
	return item
}

func TestHandleCreateItem(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"name":"M6 bolt","container_id":1,"description":"DIN 933","tags":["m6","bolt"]}`
	req := httptest.NewRequest("POST", "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	// Check Content-Type
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var item ItemResponse
	if err := json.NewDecoder(w.Body).Decode(&item); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if item.ID <= 0 {
		t.Errorf("ID = %d, want > 0", item.ID)
	}
	if item.Name != "M6 bolt" {
		t.Errorf("Name = %q, want %q", item.Name, "M6 bolt")
	}
	if item.Description == nil || *item.Description != "DIN 933" {
		t.Errorf("Description = %v, want ptr to %q", item.Description, "DIN 933")
	}
	if len(item.Tags) != 2 {
		t.Errorf("len(Tags) = %d, want 2", len(item.Tags))
	}
	if item.ContainerLabel == "" {
		t.Error("ContainerLabel is empty")
	}
	if item.CreatedAt == "" {
		t.Error("CreatedAt is empty")
	}
	if item.UpdatedAt == "" {
		t.Error("UpdatedAt is empty")
	}
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
			name:      "too many tags",
			body:      fmt.Sprintf(`{"name":"bolt","container_id":1,"tags":[%s]}`, func() string { parts := make([]string, 21); for i := range parts { parts[i] = fmt.Sprintf(`"t%d"`, i) }; return strings.Join(parts, ",") }()),
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
			req := httptest.NewRequest("POST", "/api/items", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
			}

			var resp map[string]string
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if !strings.Contains(resp["error"], tc.wantError) {
				t.Errorf("error = %q, want to contain %q", resp["error"], tc.wantError)
			}
		})
	}
}

func TestHandleCreateItemContainerNotFound(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"name":"bolt","container_id":99999,"tags":["m6"]}`
	req := httptest.NewRequest("POST", "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

func TestHandleGetItem(t *testing.T) {
	router, _ := setupTestRouter(t)
	created := createTestItem(t, router)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/items/%d", created.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var item ItemResponse
	if err := json.NewDecoder(w.Body).Decode(&item); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if item.ID != created.ID {
		t.Errorf("ID = %d, want %d", item.ID, created.ID)
	}
	if item.Name != created.Name {
		t.Errorf("Name = %q, want %q", item.Name, created.Name)
	}
}

func TestHandleGetItemNotFound(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/api/items/99999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["error"] != "item not found" {
		t.Errorf("error = %q, want %q", resp["error"], "item not found")
	}
}

func TestHandleUpdateItem(t *testing.T) {
	router, _ := setupTestRouter(t)
	created := createTestItem(t, router)

	body := `{"name":"Updated","container_id":1}`
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/items/%d", created.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var item ItemResponse
	if err := json.NewDecoder(w.Body).Decode(&item); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if item.Name != "Updated" {
		t.Errorf("Name = %q, want %q", item.Name, "Updated")
	}
}

func TestHandleDeleteItem(t *testing.T) {
	router, _ := setupTestRouter(t)
	created := createTestItem(t, router)

	// Delete the item
	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/items/%d", created.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusNoContent, w.Body.String())
	}

	// Confirm deleted
	req = httptest.NewRequest("GET", fmt.Sprintf("/api/items/%d", created.ID), nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("after delete: status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleDeleteItemNotFound(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest("DELETE", "/api/items/99999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleAddTag(t *testing.T) {
	router, _ := setupTestRouter(t)
	created := createTestItem(t, router) // has ["bolt","m6"]

	body := `{"name":"stainless"}`
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/items/%d/tags", created.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var item ItemResponse
	if err := json.NewDecoder(w.Body).Decode(&item); err != nil {
		t.Fatalf("decode: %v", err)
	}

	found := false
	for _, tag := range item.Tags {
		if tag == "stainless" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Tags = %v, want to include 'stainless'", item.Tags)
	}
	// Original tags + new one
	if len(item.Tags) < 3 {
		t.Errorf("len(Tags) = %d, want >= 3", len(item.Tags))
	}
}

func TestHandleRemoveTag(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Create item with two tags
	body := `{"name":"Remove tag test","container_id":1,"tags":["m6","bolt"]}`
	req := httptest.NewRequest("POST", "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d; body: %s", w.Code, w.Body.String())
	}
	var created ItemResponse
	json.NewDecoder(w.Body).Decode(&created)

	// Remove "m6" tag
	req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/items/%d/tags/m6", created.ID), nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("remove: status = %d, want %d; body: %s", w.Code, http.StatusNoContent, w.Body.String())
	}

	// Verify only "bolt" remains
	req = httptest.NewRequest("GET", fmt.Sprintf("/api/items/%d", created.ID), nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var item ItemResponse
	json.NewDecoder(w.Body).Decode(&item)
	if len(item.Tags) != 1 {
		t.Errorf("len(Tags) = %d, want 1", len(item.Tags))
	}
	if len(item.Tags) == 1 && item.Tags[0] != "bolt" {
		t.Errorf("Tags[0] = %q, want %q", item.Tags[0], "bolt")
	}
}

func TestHandleListContainerItems(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Create 2 items in container 1
	for i := 0; i < 2; i++ {
		body := fmt.Sprintf(`{"name":"List item %d","container_id":1,"tags":["tag%d"]}`, i, i)
		req := httptest.NewRequest("POST", "/api/items", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create item %d: status = %d; body: %s", i, w.Code, w.Body.String())
		}
	}

	req := httptest.NewRequest("GET", "/api/containers/1/items", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var result ContainerWithItems
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("len(Items) = %d, want 2", len(result.Items))
	}
	if result.Label == "" {
		t.Error("Label is empty")
	}
}

func TestHandleListItems(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Create 2 items
	for i := 0; i < 2; i++ {
		body := fmt.Sprintf(`{"name":"All item %d","container_id":1,"tags":["tag%d"]}`, i, i)
		req := httptest.NewRequest("POST", "/api/items", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create item %d: status = %d", i, w.Code)
		}
	}

	req := httptest.NewRequest("GET", "/api/items", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var items []ItemResponse
	if err := json.NewDecoder(w.Body).Decode(&items); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) < 2 {
		t.Errorf("len(items) = %d, want >= 2", len(items))
	}
}

func TestHandleListTags(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Create item with tags
	body := `{"name":"Tags list","container_id":1,"tags":["alpha","beta"]}`
	req := httptest.NewRequest("POST", "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/api/tags", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var tags []TagResponse
	if err := json.NewDecoder(w.Body).Decode(&tags); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(tags) < 2 {
		t.Errorf("len(tags) = %d, want >= 2", len(tags))
	}
}

func TestHandleListTagsPrefix(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"name":"Prefix test","container_id":1,"tags":["m6","m8","bolt"]}`
	req := httptest.NewRequest("POST", "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/api/tags?q=m", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var tags []TagResponse
	if err := json.NewDecoder(w.Body).Decode(&tags); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, tag := range tags {
		if !strings.HasPrefix(tag.Name, "m") {
			t.Errorf("tag %q does not start with 'm'", tag.Name)
		}
	}
}

func TestHandleCreateItemTagNormalization(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"name":"Normalize test","container_id":1,"tags":["M6"," Bolt "]}`
	req := httptest.NewRequest("POST", "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var item ItemResponse
	if err := json.NewDecoder(w.Body).Decode(&item); err != nil {
		t.Fatalf("decode: %v", err)
	}

	hasM6 := false
	hasBolt := false
	for _, tag := range item.Tags {
		if tag == "m6" {
			hasM6 = true
		}
		if tag == "bolt" {
			hasBolt = true
		}
	}
	if !hasM6 {
		t.Errorf("Tags %v missing 'm6' (normalized from 'M6')", item.Tags)
	}
	if !hasBolt {
		t.Errorf("Tags %v missing 'bolt' (normalized from ' Bolt ')", item.Tags)
	}
}

// --- Search handler tests ---

func TestHandleSearchByName(t *testing.T) {
	router, _ := setupTestRouter(t)
	createTestItem(t, router) // creates "Test Bolt" with tags ["m6","bolt"]

	req := httptest.NewRequest("GET", "/api/search?q=bolt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string][]ItemResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	results := resp["results"]
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Name != "Test Bolt" {
		t.Errorf("Name = %q, want %q", results[0].Name, "Test Bolt")
	}
}

func TestHandleSearchByTag(t *testing.T) {
	router, _ := setupTestRouter(t)
	createTestItem(t, router) // creates "Test Bolt" with tags ["m6","bolt"]

	req := httptest.NewRequest("GET", "/api/search?q=m6", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string][]ItemResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp["results"]) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(resp["results"]))
	}
}

func TestHandleSearchEmpty(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/api/search?q=", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string][]ItemResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp["results"]) != 0 {
		t.Errorf("len(results) = %d, want 0", len(resp["results"]))
	}
}

func TestHandleSearchMissingParam(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/api/search", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string][]ItemResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp["results"]) != 0 {
		t.Errorf("len(results) = %d, want 0", len(resp["results"]))
	}
}

func TestHandleSearchResponseShape(t *testing.T) {
	router, _ := setupTestRouter(t)
	createTestItem(t, router) // creates "Test Bolt" with tags ["m6","bolt"]

	req := httptest.NewRequest("GET", "/api/search?q=bolt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := raw["results"]; !ok {
		t.Fatal("response missing 'results' key")
	}

	var results []ItemResponse
	if err := json.Unmarshal(raw["results"], &results); err != nil {
		t.Fatalf("decode results: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("results is empty, need at least 1 to verify shape")
	}

	r := results[0]
	if r.ID <= 0 {
		t.Error("ID missing")
	}
	if r.Name == "" {
		t.Error("Name missing")
	}
	if r.ContainerLabel == "" {
		t.Error("ContainerLabel missing")
	}
	if r.Tags == nil {
		t.Error("Tags is nil")
	}
}

func TestHandleSearchCaseInsensitive(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Create an item with uppercase name
	body := `{"name":"BOLT","container_id":1,"tags":["steel"]}`
	req := httptest.NewRequest("POST", "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d; body: %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("GET", "/api/search?q=bolt", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string][]ItemResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp["results"]) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(resp["results"]))
	}
	if resp["results"][0].Name != "BOLT" {
		t.Errorf("Name = %q, want %q", resp["results"][0].Name, "BOLT")
	}
}

func TestHandleCreateItemDescriptionOptional(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"name":"Nut","container_id":1,"tags":["m6"]}`
	req := httptest.NewRequest("POST", "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var item ItemResponse
	if err := json.NewDecoder(w.Body).Decode(&item); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if item.Description != nil {
		t.Errorf("Description = %v, want nil", item.Description)
	}
}

func TestErrorResponseFormat(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/api/items/99999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	errMsg, ok := resp["error"]
	if !ok {
		t.Fatal("response missing 'error' key")
	}
	if errMsg == "" {
		t.Error("error message is empty")
	}
}

// --- Resize handler tests ---

func TestHandleResizeShelf_Success(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"rows":8,"cols":12}`
	req := httptest.NewRequest("PUT", "/api/shelf/resize", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if int(result["rows"].(float64)) != 8 {
		t.Errorf("rows = %v, want 8", result["rows"])
	}
	if int(result["cols"].(float64)) != 12 {
		t.Errorf("cols = %v, want 12", result["cols"])
	}
}

func TestHandleResizeShelf_Conflict(t *testing.T) {
	router, store := setupTestRouter(t)

	// Create item in a high-coordinate container
	var containerID int64
	err := store.db.QueryRow("SELECT id FROM container WHERE col = 10 AND row = 5").Scan(&containerID)
	if err != nil {
		t.Fatalf("get container: %v", err)
	}
	body := fmt.Sprintf(`{"name":"Test Bolt","container_id":%d,"tags":["test"]}`, containerID)
	req := httptest.NewRequest("POST", "/api/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create item: status = %d; body: %s", w.Code, w.Body.String())
	}

	// Try resize to 3x3 — should conflict because (10,5) is outside bounds
	body2 := `{"rows":3,"cols":3}`
	req = httptest.NewRequest("PUT", "/api/shelf/resize", strings.NewReader(body2))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusConflict, w.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["blocked"] != true {
		t.Errorf("blocked = %v, want true", result["blocked"])
	}
	affected, ok := result["affected"].([]any)
	if !ok || len(affected) == 0 {
		t.Fatalf("affected missing or empty: %v", result["affected"])
	}
}

func TestHandleResizeShelf_BadRequest_InvalidJSON(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest("PUT", "/api/shelf/resize", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestValidateResize_TooSmall(t *testing.T) {
	req := &ResizeRequest{Rows: 0, Cols: 5}
	msg := validateResizeRequest(req)
	if msg != "rows must be between 1 and 26" {
		t.Errorf("msg = %q, want rows validation error", msg)
	}
}

func TestValidateResize_TooLarge(t *testing.T) {
	req := &ResizeRequest{Rows: 5, Cols: 31}
	msg := validateResizeRequest(req)
	if msg != "cols must be between 1 and 30" {
		t.Errorf("msg = %q, want cols validation error", msg)
	}
}

func TestValidateResize_Valid(t *testing.T) {
	req := &ResizeRequest{Rows: 5, Cols: 10}
	msg := validateResizeRequest(req)
	if msg != "" {
		t.Errorf("msg = %q, want empty (valid)", msg)
	}
}

func TestHandleResizeShelf_NameUpdate(t *testing.T) {
	router, store := setupTestRouter(t)

	body := `{"rows":5,"cols":10,"name":"Workshop"}`
	req := httptest.NewRequest("PUT", "/api/shelf/resize", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Verify name was updated in the database
	var name string
	err := store.db.QueryRow("SELECT name FROM shelf LIMIT 1").Scan(&name)
	if err != nil {
		t.Fatalf("query shelf name: %v", err)
	}
	if name != "Workshop" {
		t.Errorf("shelf name = %q, want %q", name, "Workshop")
	}
}
