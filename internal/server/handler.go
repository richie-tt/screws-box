package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"screws-box/internal/model"
	oidcpkg "screws-box/internal/oidc"
	"screws-box/internal/store"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"
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
	SearchItemsBatch(ctx context.Context, query string, tags []string) (*model.SearchResponse, error)
	ListTags(ctx context.Context, prefix string) ([]model.TagResponse, error)
	RenameTag(ctx context.Context, tagID int64, newName string) error
	MergeTags(ctx context.Context, sourceID, targetID int64) error
	DeleteUnusedTag(ctx context.Context, tagID int64) error
	ResizeShelf(ctx context.Context, newRows, newCols int) (*model.ResizeResult, error)
	UpdateShelfName(ctx context.Context, name string) error
	GetAuthSettings(ctx context.Context) (*model.AuthSettings, error)
	UpdateAuthSettings(ctx context.Context, settings *model.AuthSettings) error
	ValidateCredentials(ctx context.Context, username, password string) (bool, error)
	GetOIDCConfig(ctx context.Context) (*model.OIDCConfig, error)
	GetOIDCConfigMasked(ctx context.Context) (*model.OIDCConfig, error)
	SaveOIDCConfig(ctx context.Context, cfg *model.OIDCConfig) error
	UpsertOIDCUser(ctx context.Context, user *model.OIDCUser) (*model.OIDCUser, error)
	GetOIDCUserBySub(ctx context.Context, sub, issuer string) (*model.OIDCUser, error)
	GetOrCreateEncryptionKey(ctx context.Context) ([]byte, error)
	ExportAllData(ctx context.Context) (*model.ExportData, error)
	ImportAllData(ctx context.Context, data *model.ExportData) error
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

// getDisplayName returns the display name for the current session user.
func (srv *Server) getDisplayName(r *http.Request) string {
	sess := srv.sessions.GetSession(r)
	if sess == nil {
		return ""
	}
	if sess.DisplayName != "" {
		return sess.DisplayName
	}
	return sess.Username
}

func (srv *Server) handleGrid() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := srv.store.GetGridData()
		if err != nil {
			slog.Error("failed to load grid data", "err", err)
			data = &model.GridData{Error: "Cannot load shelf -- check server logs."}
		}
		data.DisplayName = srv.getDisplayName(r)

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

// SettingsData is the view model for the settings template.
type SettingsData struct {
	ShelfName        string
	Rows             int
	Cols             int
	AuthEnabled      bool
	AuthUser         string
	AuthHasPassword  bool
	DisplayName      string
	OIDCEnabled      bool
	OIDCIssuer       string
	OIDCClientID     string
	OIDCDisplayName  string
	OIDCSecretStatus string
	SessionStoreType string
	SessionCount     int
	Sessions         []SessionInfo
	CurrentSessionID string
	SessionTTL       int64 // TTL in seconds for JS expiry calculation
}

func (srv *Server) handleSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := srv.store.GetGridData()
		if err != nil {
			slog.Error("failed to load settings data", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		settingsData := SettingsData{
			ShelfName:       data.ShelfName,
			Rows:            data.Rows,
			Cols:            data.Cols,
			AuthEnabled:     data.AuthEnabled,
			AuthUser:        data.AuthUser,
			AuthHasPassword: data.AuthHasPassword,
			DisplayName:     srv.getDisplayName(r),
		}
		oidcCfg, oidcErr := srv.store.GetOIDCConfigMasked(r.Context())
		if oidcErr == nil {
			settingsData.OIDCEnabled = oidcCfg.Enabled
			settingsData.OIDCIssuer = oidcCfg.IssuerURL
			settingsData.OIDCClientID = oidcCfg.ClientID
			settingsData.OIDCDisplayName = oidcCfg.DisplayName
			settingsData.OIDCSecretStatus = oidcCfg.SecretStatus
		}

		// Populate session data for settings template
		sessions, _ := srv.sessions.ListSessions(r.Context())
		currentSess := srv.sessions.GetSession(r)
		currentSessID := ""
		if currentSess != nil {
			currentSessID = currentSess.ID
		}
		var sessionInfos []SessionInfo
		for _, s := range sessions {
			sessionInfos = append(sessionInfos, mapSessionToInfo(s, currentSessID, srv.sessions.TTL()))
		}
		sortSessionInfos(sessionInfos)
		settingsData.SessionStoreType = srv.sessions.StoreType()
		settingsData.SessionCount = len(sessionInfos)
		settingsData.Sessions = sessionInfos
		settingsData.CurrentSessionID = currentSessID
		settingsData.SessionTTL = int64(srv.sessions.TTL().Seconds())

		tmpl, err := template.ParseFS(mustSubFS(ContentFS, "templates"),
			"layout.html", "settings.html")
		if err != nil {
			slog.Error("settings template parse error", "err", err)
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, settingsData); err != nil {
			slog.Error("settings template execute error", "err", err)
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

		// When no query and no tags, return empty response.
		if q == "" && len(tags) == 0 {
			writeJSON(w, http.StatusOK, &model.SearchResponse{Results: []model.SearchResult{}, TotalCount: 0})
			return
		}

		result, err := srv.store.SearchItemsBatch(r.Context(), q, tags)
		if err != nil {
			slog.Error("search items", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, result)
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

// RenameTagRequest is the request body for PUT /api/tags/{tagID}.
type RenameTagRequest struct {
	Name       string `json:"name"`
	ForceMerge bool   `json:"force_merge,omitempty"`
}

// findTagByName returns the first tag with an exact name match, or nil.
func (srv *Server) findTagByName(ctx context.Context, name string) (*model.TagResponse, error) {
	tags, err := srv.store.ListTags(ctx, name)
	if err != nil {
		return nil, err
	}
	for _, t := range tags {
		if t.Name == name {
			return &t, nil
		}
	}
	return nil, nil
}

// findTagByID returns the tag with the given ID, or nil.
func (srv *Server) findTagByID(ctx context.Context, id int64) (*model.TagResponse, error) {
	tags, err := srv.store.ListTags(ctx, "")
	if err != nil {
		return nil, err
	}
	for _, t := range tags {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, nil
}

func (srv *Server) handleRenameTag() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tagIDStr := chi.URLParam(r, "tagID")
		tagID, err := strconv.ParseInt(tagIDStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid tag ID")
			return
		}

		var req RenameTagRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		// Validate tag name (same rules as validateAddTag)
		req.Name = strings.ToLower(strings.TrimSpace(req.Name))
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "tag must not be empty")
			return
		}
		if len(req.Name) > 50 {
			writeError(w, http.StatusBadRequest, "tag must be at most 50 characters")
			return
		}

		ctx := r.Context()

		if req.ForceMerge {
			// Look up target tag by name
			target, err := srv.findTagByName(ctx, req.Name)
			if err != nil {
				slog.Error("find target tag", "err", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			if target == nil {
				writeError(w, http.StatusNotFound, "target tag not found")
				return
			}
			if err := srv.store.MergeTags(ctx, tagID, target.ID); err != nil {
				slog.Error("merge tags", "err", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}

		// Try rename
		if err := srv.store.RenameTag(ctx, tagID, req.Name); err != nil {
			// Check for UNIQUE constraint violation (name conflict)
			if strings.Contains(err.Error(), "UNIQUE constraint") {
				// Look up existing tag with that name
				target, lookupErr := srv.findTagByName(ctx, req.Name)
				if lookupErr != nil {
					slog.Error("find conflicting tag", "err", lookupErr)
					writeError(w, http.StatusInternalServerError, "internal error")
					return
				}
				source, lookupErr := srv.findTagByID(ctx, tagID)
				if lookupErr != nil {
					slog.Error("find source tag", "err", lookupErr)
					writeError(w, http.StatusInternalServerError, "internal error")
					return
				}
				writeJSON(w, http.StatusConflict, map[string]any{
					"merge_needed": true,
					"target":       target,
					"source":       source,
				})
				return
			}
			slog.Error("rename tag", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

func (srv *Server) handleDeleteTag() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tagIDStr := chi.URLParam(r, "tagID")
		tagID, err := strconv.ParseInt(tagIDStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid tag ID")
			return
		}

		if err := srv.store.DeleteUnusedTag(r.Context(), tagID); err != nil {
			if errors.Is(err, store.ErrTagInUse) {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "tag is in use"})
				return
			}
			slog.Error("delete tag", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		w.WriteHeader(http.StatusNoContent)
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
	Error           string
	OIDCEnabled     bool
	OIDCDisplayName string
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
		data := loginData{}
		// Check for error query params from OIDC callbacks
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			switch errParam {
			case "sso_unavailable":
				data.Error = "SSO provider is unreachable. Use username and password to sign in."
			default:
				data.Error = "Authentication failed. Please try again."
			}
		}
		// Load OIDC config to conditionally show SSO button
		oidcCfg, oidcErr := srv.store.GetOIDCConfig(r.Context())
		if oidcErr == nil && oidcCfg.Enabled && oidcCfg.IssuerURL != "" {
			data.OIDCEnabled = true
			data.OIDCDisplayName = oidcCfg.DisplayName
			if data.OIDCDisplayName == "" {
				data.OIDCDisplayName = "SSO"
			}
		}
		renderLogin(w, data)
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

func (srv *Server) handleOIDCStart() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		// Load OIDC config
		cfg, err := srv.store.GetOIDCConfig(ctx)
		if err != nil || !cfg.Enabled {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		// Get encryption key
		key, err := srv.store.GetOrCreateEncryptionKey(ctx)
		if err != nil {
			slog.Error("oidc start: get encryption key", "err", err)
			http.Redirect(w, r, "/login?error=auth_failed", http.StatusFound)
			return
		}
		// Build callback URL from request
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		callbackURL := fmt.Sprintf("%s://%s/auth/callback", scheme, r.Host)

		// Create OIDC provider
		provider, err := oidcpkg.NewProviderFromConfig(ctx, cfg.IssuerURL, cfg.ClientID, cfg.ClientSecret, callbackURL)
		if err != nil {
			slog.Error("oidc start: create provider", "err", err, "issuer", cfg.IssuerURL)
			http.Redirect(w, r, "/login?error=sso_unavailable", http.StatusFound)
			return
		}
		// Generate PKCE verifier, state, nonce
		verifier := oauth2.GenerateVerifier()
		state := oidcpkg.GenerateState()
		nonce := oidcpkg.GenerateNonce()

		// Encrypt state cookie
		cookieValue, err := oidcpkg.EncryptStateCookie(key, &oidcpkg.StateCookie{
			State:    state,
			Nonce:    nonce,
			Verifier: verifier,
		})
		if err != nil {
			slog.Error("oidc start: encrypt state cookie", "err", err)
			http.Redirect(w, r, "/login?error=auth_failed", http.StatusFound)
			return
		}
		// Set state cookie and redirect to provider
		secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
		http.SetCookie(w, oidcpkg.MakeStateCookieHTTP(cookieValue, secure))
		authURL := provider.AuthURL(state, nonce, verifier)
		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

func (srv *Server) handleOIDCCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		// Check for error from provider
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			slog.Warn("oidc callback: provider error", "error", errParam, "desc", r.URL.Query().Get("error_description"))
			http.Redirect(w, r, "/login?error=auth_failed", http.StatusFound)
			return
		}
		code := r.URL.Query().Get("code")
		stateParam := r.URL.Query().Get("state")
		if code == "" || stateParam == "" {
			http.Redirect(w, r, "/login?error=auth_failed", http.StatusFound)
			return
		}
		// Get encryption key
		key, err := srv.store.GetOrCreateEncryptionKey(ctx)
		if err != nil {
			slog.Error("oidc callback: get encryption key", "err", err)
			http.Redirect(w, r, "/login?error=auth_failed", http.StatusFound)
			return
		}
		// Decrypt state cookie
		stateCookie, err := r.Cookie(oidcpkg.StateCookieName)
		if err != nil {
			slog.Warn("oidc callback: state cookie missing")
			http.Redirect(w, r, "/login?error=auth_failed", http.StatusFound)
			return
		}
		sc, err := oidcpkg.DecryptStateCookie(key, stateCookie.Value)
		if err != nil {
			slog.Warn("oidc callback: decrypt state cookie failed", "err", err)
			http.Redirect(w, r, "/login?error=auth_failed", http.StatusFound)
			return
		}
		// Verify state matches (CSRF protection)
		if sc.State != stateParam {
			slog.Warn("oidc callback: state mismatch")
			http.Redirect(w, r, "/login?error=auth_failed", http.StatusFound)
			return
		}
		// Clear state cookie
		secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
		http.SetCookie(w, oidcpkg.ClearStateCookieHTTP(secure))

		// Load OIDC config
		cfg, err := srv.store.GetOIDCConfig(ctx)
		if err != nil || !cfg.Enabled {
			http.Redirect(w, r, "/login?error=auth_failed", http.StatusFound)
			return
		}
		// Build callback URL (same derivation as start handler)
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		callbackURL := fmt.Sprintf("%s://%s/auth/callback", scheme, r.Host)

		// Create provider and exchange code
		provider, err := oidcpkg.NewProviderFromConfig(ctx, cfg.IssuerURL, cfg.ClientID, cfg.ClientSecret, callbackURL)
		if err != nil {
			slog.Error("oidc callback: create provider", "err", err)
			http.Redirect(w, r, "/login?error=sso_unavailable", http.StatusFound)
			return
		}
		claims, err := provider.ExchangeAndVerify(ctx, code, sc.Verifier, sc.Nonce)
		if err != nil {
			slog.Error("oidc callback: exchange and verify", "err", err)
			http.Redirect(w, r, "/login?error=auth_failed", http.StatusFound)
			return
		}

		// Determine username: email fallback to sub
		username := claims.Email
		if username == "" {
			username = claims.Sub
		}
		// Upsert OIDC user record
		_, err = srv.store.UpsertOIDCUser(ctx, &model.OIDCUser{
			Sub:         claims.Sub,
			Issuer:      claims.Issuer,
			Email:       claims.Email,
			DisplayName: claims.DisplayName,
			AvatarURL:   claims.AvatarURL,
		})
		if err != nil {
			slog.Error("oidc callback: upsert user", "err", err)
			// Non-fatal: continue with session creation
		}
		// Create session with AuthMethod="oidc" and DisplayName
		if err := srv.sessions.CreateWithMethod(w, r, username, "oidc", claims.DisplayName); err != nil {
			slog.Error("oidc callback: create session", "err", err)
			http.Redirect(w, r, "/login?error=auth_failed", http.StatusFound)
			return
		}
		// Redirect to grid
		http.Redirect(w, r, "/", http.StatusFound)
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

// --- OIDC Config API ---

func (srv *Server) handleGetOIDCConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := srv.store.GetOIDCConfigMasked(r.Context())
		if err != nil {
			slog.Error("get oidc config", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, cfg)
	}
}

type updateOIDCConfigRequest struct {
	Enabled      bool   `json:"enabled"`
	IssuerURL    string `json:"issuer_url"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	DisplayName  string `json:"display_name"`
}

func (srv *Server) handleUpdateOIDCConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req updateOIDCConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		req.IssuerURL = strings.TrimSpace(req.IssuerURL)
		req.ClientID = strings.TrimSpace(req.ClientID)
		req.DisplayName = strings.TrimSpace(req.DisplayName)

		ctx := r.Context()

		// Check if OIDC was previously enabled (for session revocation D-22)
		prevCfg, _ := srv.store.GetOIDCConfig(ctx)
		wasEnabled := prevCfg != nil && prevCfg.Enabled

		if req.Enabled {
			// Validate required fields when enabling
			if req.IssuerURL == "" || req.ClientID == "" {
				writeError(w, http.StatusBadRequest, "Issuer URL, Client ID, and Client Secret are required.")
				return
			}
			// Check if secret is required (no existing secret and none provided)
			if req.ClientSecret == "" {
				existingCfg, _ := srv.store.GetOIDCConfig(ctx)
				if existingCfg == nil || existingCfg.ClientSecret == "" {
					writeError(w, http.StatusBadRequest, "Issuer URL, Client ID, and Client Secret are required.")
					return
				}
			}
			// Validate provider by hitting discovery endpoint (D-21)
			if err := oidcpkg.ValidateDiscovery(ctx, req.IssuerURL); err != nil {
				slog.Warn("oidc config: discovery validation failed", "issuer", req.IssuerURL, "err", err)
				writeError(w, http.StatusBadRequest, fmt.Sprintf("Could not reach OIDC provider at %s. Check the Issuer URL.", req.IssuerURL))
				return
			}
		}

		// Save config
		cfg := &model.OIDCConfig{
			Enabled:      req.Enabled,
			IssuerURL:    req.IssuerURL,
			ClientID:     req.ClientID,
			ClientSecret: req.ClientSecret, // Store handles empty = preserve existing
			DisplayName:  req.DisplayName,
		}
		if err := srv.store.SaveOIDCConfig(ctx, cfg); err != nil {
			slog.Error("save oidc config", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// If OIDC was disabled (was enabled, now not), revoke OIDC sessions (D-22)
		if wasEnabled && !req.Enabled {
			count, err := srv.sessions.DeleteByAuthMethod(ctx, "oidc")
			if err != nil {
				slog.Error("revoke oidc sessions", "err", err)
			} else {
				slog.Info("revoked oidc sessions on disable", "count", count)
			}
		}

		// Return updated masked config
		updated, err := srv.store.GetOIDCConfigMasked(ctx)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
		writeJSON(w, http.StatusOK, updated)
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
