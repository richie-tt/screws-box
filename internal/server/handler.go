package server

import (
	"context"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"screws-box/internal/model"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// StoreService defines the storage operations required by HTTP handlers.
type StoreService interface {
	Ping(ctx context.Context) error
	GetGridData() (*model.GridData, error)
	CreateItem(ctx context.Context, containerID int64, name string, description *string, tags []string) (*model.ItemResponse, error)
	GetItem(ctx context.Context, id int64) (*model.ItemResponse, error)
	UpdateItem(ctx context.Context, id int64, name string, description *string, containerID int64) (*model.ItemResponse, error)
	DeleteItem(ctx context.Context, id int64) error
	AddTagToItem(ctx context.Context, itemID int64, tagName string) (*model.ItemResponse, error)
	RemoveTagFromItem(ctx context.Context, itemID int64, tagName string) error
	ListItemsByContainer(ctx context.Context, containerID int64) (*model.ContainerWithItems, error)
	ListAllItems(ctx context.Context) ([]model.ItemResponse, error)
	SearchItems(ctx context.Context, query string) ([]model.ItemResponse, error)
	SearchItemsByTags(ctx context.Context, query string, tags []string) ([]model.ItemResponse, error)
	ListTags(ctx context.Context, prefix string) ([]model.TagResponse, error)
	ResizeShelf(ctx context.Context, newRows, newCols int) (*model.ResizeResult, error)
	UpdateShelfName(ctx context.Context, name string) error
	GetAuthSettings(ctx context.Context) (*model.AuthSettings, error)
	UpdateAuthSettings(ctx context.Context, settings *model.AuthSettings) error
	ValidateCredentials(ctx context.Context, username, password string) (bool, error)
}

// --- Healthcheck ---

func (srv *Server) handleHealthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := srv.store.Ping(r.Context()); err != nil {
			slog.Error("healthcheck failed", "err", err)
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON encode failed", "err", err)
	}
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
	req.Tags = model.Dedup(req.Tags)
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

// ValidateResizeRequest validates a resize request. Exported for handler tests.
func ValidateResizeRequest(req *model.ResizeRequest) string {
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

func (srv *Server) handleResizeShelf() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req model.ResizeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if msg := ValidateResizeRequest(&req); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		result, err := srv.store.ResizeShelf(r.Context(), req.Rows, req.Cols)
		if err != nil {
			slog.Error("resize shelf", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if result.Blocked {
			writeJSON(w, http.StatusConflict, result)
			return
		}
		if req.Name != nil {
			if err := srv.store.UpdateShelfName(r.Context(), *req.Name); err != nil {
				slog.Error("update shelf name", "err", err)
			}
		}
		writeJSON(w, http.StatusOK, result)
	}
}

// --- Template handler ---

func (srv *Server) handleGrid() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		data, err := srv.store.GetGridData()
		if err != nil {
			slog.Error("failed to load grid data", "err", err)
			data = &model.GridData{Error: "Cannot load shelf -- check server logs."}
		}

		tmpl, err := template.ParseFS(mustSubFS(ContentFS, "templates"),
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

func (srv *Server) handleCreateItem() http.HandlerFunc {
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
		item, err := srv.store.CreateItem(r.Context(), req.ContainerID, req.Name, req.Description, req.Tags)
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

func (srv *Server) handleGetItem() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "itemID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid item ID")
			return
		}
		item, err := srv.store.GetItem(r.Context(), id)
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

func (srv *Server) handleUpdateItem() http.HandlerFunc {
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
		item, err := srv.store.UpdateItem(r.Context(), id, req.Name, req.Description, req.ContainerID)
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

func (srv *Server) handleDeleteItem() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "itemID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid item ID")
			return
		}
		if err := srv.store.DeleteItem(r.Context(), id); err != nil {
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

func (srv *Server) handleAddTag() http.HandlerFunc {
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
		item, err := srv.store.AddTagToItem(r.Context(), id, req.Name)
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

func (srv *Server) handleRemoveTag() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "itemID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid item ID")
			return
		}
		tagName := strings.ToLower(chi.URLParam(r, "tagName"))
		if err := srv.store.RemoveTagFromItem(r.Context(), id, tagName); err != nil {
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

func (srv *Server) handleListContainerItems() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "containerID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid container ID")
			return
		}
		result, err := srv.store.ListItemsByContainer(r.Context(), id)
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

func (srv *Server) handleListItems() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := srv.store.ListAllItems(r.Context())
		if err != nil {
			slog.Error("list items", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, items)
	}
}

func (srv *Server) handleSearch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
		tagParam := strings.TrimSpace(r.URL.Query().Get("tags"))

		var tags []string
		if tagParam != "" {
			for _, t := range strings.Split(tagParam, ",") {
				t = strings.ToLower(strings.TrimSpace(t))
				if t != "" {
					tags = append(tags, t)
				}
			}
		}

		// When tags are active but no text query, search by tags only.
		if q == "" && len(tags) == 0 {
			writeJSON(w, http.StatusOK, map[string][]model.ItemResponse{"results": {}})
			return
		}

		var items []model.ItemResponse
		var err error
		if len(tags) > 0 {
			items, err = srv.store.SearchItemsByTags(r.Context(), q, tags)
		} else {
			items, err = srv.store.SearchItems(r.Context(), q)
		}
		if err != nil {
			slog.Error("search items", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"results": items})
	}
}

func (srv *Server) handleListTags() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
		tags, err := srv.store.ListTags(r.Context(), q)
		if err != nil {
			slog.Error("list tags", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, tags)
	}
}

// authMiddleware checks if authentication is enabled and redirects to /login if needed.
func (srv *Server) authMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			settings, err := srv.store.GetAuthSettings(r.Context())
			if err != nil {
				slog.Error("auth middleware: get settings", "err", err)
				next.ServeHTTP(w, r)
				return
			}
			if !settings.Enabled {
				next.ServeHTTP(w, r)
				return
			}
			if srv.sessions.GetUser(r) != "" {
				next.ServeHTTP(w, r)
				return
			}
			// Not authenticated — redirect HTML requests, 401 for API
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
		})
	}
}

type loginData struct {
	Error string
}

func (srv *Server) handleLoginPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// If auth not enabled, redirect to home
		settings, err := srv.store.GetAuthSettings(r.Context())
		if err == nil && !settings.Enabled {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		// If already logged in, redirect to home
		if srv.sessions.GetUser(r) != "" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		renderLogin(w, loginData{})
	}
}

func (srv *Server) handleLoginPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")

		valid, err := srv.store.ValidateCredentials(r.Context(), username, password)
		if err != nil {
			slog.Error("login: validate credentials", "err", err)
			renderLogin(w, loginData{Error: "Internal error, please try again."})
			return
		}
		if !valid {
			renderLogin(w, loginData{Error: "Invalid username or password."})
			return
		}
		if err := srv.sessions.Create(w, r, username); err != nil {
			slog.Error("login: create session", "err", err)
			renderLogin(w, loginData{Error: "Internal error, please try again."})
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func (srv *Server) handleLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		srv.sessions.Destroy(w, r)
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func renderLogin(w http.ResponseWriter, data loginData) {
	tmpl, err := template.ParseFS(mustSubFS(ContentFS, "templates"),
		"layout.html", "login.html")
	if err != nil {
		slog.Error("login template parse error", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		slog.Error("login template execute error", "err", err)
	}
}

func (srv *Server) handleGetAuthSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		settings, err := srv.store.GetAuthSettings(r.Context())
		if err != nil {
			slog.Error("get auth settings", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, settings)
	}
}

func validatePassword(pw string) string {
	if len(pw) < 12 {
		return "password must be at least 12 characters"
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, c := range pw {
		switch {
		case 'A' <= c && c <= 'Z':
			hasUpper = true
		case 'a' <= c && c <= 'z':
			hasLower = true
		case '0' <= c && c <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	if !hasUpper {
		return "password must contain an uppercase letter"
	}
	if !hasLower {
		return "password must contain a lowercase letter"
	}
	if !hasDigit {
		return "password must contain a digit"
	}
	if !hasSpecial {
		return "password must contain a special character"
	}
	return ""
}

func (srv *Server) handleUpdateAuthSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req model.AuthSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		req.Username = strings.TrimSpace(req.Username)
		req.Password = strings.TrimSpace(req.Password)
		if req.Enabled && req.Username == "" {
			writeError(w, http.StatusBadRequest, "username is required when auth is enabled")
			return
		}
		// Validate password strength when provided
		if req.Password != "" {
			if msg := validatePassword(req.Password); msg != "" {
				writeError(w, http.StatusBadRequest, msg)
				return
			}
		}
		// When enabling auth, require a password if none has been set yet
		if req.Enabled && req.Password == "" {
			existing, err := srv.store.GetAuthSettings(r.Context())
			if err != nil || !existing.HasPassword {
				writeError(w, http.StatusBadRequest, "password is required when auth is enabled")
				return
			}
		}
		if err := srv.store.UpdateAuthSettings(r.Context(), &req); err != nil {
			slog.Error("update auth settings", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		// Return updated settings (without password)
		updated, err := srv.store.GetAuthSettings(r.Context())
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
		writeJSON(w, http.StatusOK, updated)
	}
}
