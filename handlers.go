package main

import (
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// --- Request types ---

type CreateItemRequest struct {
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	ContainerID int64    `json:"container_id"`
	Tags        []string `json:"tags"`
}

type UpdateItemRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	ContainerID int64   `json:"container_id"`
}

type AddTagRequest struct {
	Name string `json:"name"`
}

// --- Validation ---

func validateCreateItem(req *CreateItemRequest) string {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return "name is required"
	}
	if len(req.Name) > 200 {
		return "name must be at most 200 characters"
	}
	if req.ContainerID <= 0 {
		return "container_id is required"
	}
	if len(req.Tags) == 0 {
		return "at least one tag is required"
	}
	if len(req.Tags) > 20 {
		return "at most 20 tags allowed"
	}
	for i, t := range req.Tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			return "tag must not be empty"
		}
		if len(t) > 50 {
			return "tag must be at most 50 characters"
		}
		req.Tags[i] = t
	}
	req.Tags = dedup(req.Tags)
	if req.Description != nil && len(*req.Description) > 1000 {
		return "description must be at most 1000 characters"
	}
	return ""
}

func validateUpdateItem(req *UpdateItemRequest) string {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return "name is required"
	}
	if len(req.Name) > 200 {
		return "name must be at most 200 characters"
	}
	if req.ContainerID <= 0 {
		return "container_id is required"
	}
	if req.Description != nil && len(*req.Description) > 1000 {
		return "description must be at most 1000 characters"
	}
	return ""
}

func validateAddTag(req *AddTagRequest) string {
	req.Name = strings.ToLower(strings.TrimSpace(req.Name))
	if req.Name == "" {
		return "tag must not be empty"
	}
	if len(req.Name) > 50 {
		return "tag must be at most 50 characters"
	}
	return ""
}

func validateResizeRequest(req *ResizeRequest) string {
	if req.Rows < 1 || req.Rows > 26 {
		return "rows must be between 1 and 26"
	}
	if req.Cols < 1 || req.Cols > 30 {
		return "cols must be between 1 and 30"
	}
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		req.Name = &trimmed
		if len(trimmed) > 100 {
			return "name must be at most 100 characters"
		}
	}
	return ""
}

func handleResizeShelf(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ResizeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if msg := validateResizeRequest(&req); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		result, err := store.ResizeShelf(r.Context(), req.Rows, req.Cols)
		if err != nil {
			slog.Error("resize shelf", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if result.Blocked {
			writeJSON(w, http.StatusConflict, result)
			return
		}
		// If name provided, update shelf name.
		if req.Name != nil && *req.Name != "" {
			if err := store.UpdateShelfName(r.Context(), *req.Name); err != nil {
				slog.Error("update shelf name", "err", err)
			}
		}
		writeJSON(w, http.StatusOK, result)
	}
}

// --- Template handler ---

func handleGrid(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := store.GetGridData()
		if err != nil {
			slog.Error("failed to load grid data", "err", err)
			data = &GridData{Error: "Cannot load shelf -- check server logs."}
		}

		tmpl, err := template.ParseFS(mustSubFS(contentFS, "templates"),
			"layout.html", "grid.html")
		if err != nil {
			slog.Error("template parse error", "err", err)
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
			slog.Error("template execute error", "err", err)
		}
	}
}

// --- API handlers ---

func handleCreateItem(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateItemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if msg := validateCreateItem(&req); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		item, err := store.CreateItem(r.Context(), req.ContainerID, req.Name, req.Description, req.Tags)
		if err != nil {
			if strings.Contains(err.Error(), "container not found") {
				writeError(w, http.StatusNotFound, "container not found")
				return
			}
			slog.Error("create item", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusCreated, item)
	}
}

func handleGetItem(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "itemID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid item ID")
			return
		}
		item, err := store.GetItem(r.Context(), id)
		if err != nil {
			slog.Error("get item", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if item == nil {
			writeError(w, http.StatusNotFound, "item not found")
			return
		}
		writeJSON(w, http.StatusOK, item)
	}
}

func handleUpdateItem(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "itemID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid item ID")
			return
		}
		var req UpdateItemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if msg := validateUpdateItem(&req); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		item, err := store.UpdateItem(r.Context(), id, req.Name, req.Description, req.ContainerID)
		if err != nil {
			if strings.Contains(err.Error(), "container not found") {
				writeError(w, http.StatusNotFound, "container not found")
				return
			}
			slog.Error("update item", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if item == nil {
			writeError(w, http.StatusNotFound, "item not found")
			return
		}
		writeJSON(w, http.StatusOK, item)
	}
}

func handleDeleteItem(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "itemID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid item ID")
			return
		}
		if err := store.DeleteItem(r.Context(), id); err != nil {
			if strings.Contains(err.Error(), "item not found") {
				writeError(w, http.StatusNotFound, "item not found")
				return
			}
			slog.Error("delete item", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleAddTag(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "itemID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid item ID")
			return
		}
		var req AddTagRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if msg := validateAddTag(&req); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		item, err := store.AddTagToItem(r.Context(), id, req.Name)
		if err != nil {
			if strings.Contains(err.Error(), "item not found") {
				writeError(w, http.StatusNotFound, "item not found")
				return
			}
			slog.Error("add tag", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, item)
	}
}

func handleRemoveTag(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "itemID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid item ID")
			return
		}
		tagName := strings.ToLower(chi.URLParam(r, "tagName"))
		if err := store.RemoveTagFromItem(r.Context(), id, tagName); err != nil {
			if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not associated") {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			slog.Error("remove tag", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleListContainerItems(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "containerID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid container ID")
			return
		}
		result, err := store.ListItemsByContainer(r.Context(), id)
		if err != nil {
			slog.Error("list container items", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if result == nil {
			writeError(w, http.StatusNotFound, "container not found")
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handleListItems(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := store.ListAllItems(r.Context())
		if err != nil {
			slog.Error("list items", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, items)
	}
}

func handleSearch(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
		if q == "" {
			writeJSON(w, http.StatusOK, map[string][]ItemResponse{"results": {}})
			return
		}

		items, err := store.SearchItems(r.Context(), q)
		if err != nil {
			slog.Error("search items", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"results": items})
	}
}

func handleListTags(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
		tags, err := store.ListTags(r.Context(), q)
		if err != nil {
			slog.Error("list tags", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, tags)
	}
}
