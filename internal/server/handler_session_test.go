package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"screws-box/internal/session"
	"screws-box/internal/store"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupSessionTestRouter creates a test server with a MemoryStore and returns the router,
// the session manager, and the memory store so tests can create sessions directly.
func setupSessionTestRouter(t *testing.T) (http.Handler, *session.Manager, *session.MemoryStore) { //nolint:unparam // all returns kept for future test flexibility
	t.Helper()
	s := &store.Store{}
	tmpFile := filepath.Join(t.TempDir(), "test.db")
	require.NoError(t, s.Open(tmpFile))
	t.Cleanup(func() { s.Close() })

	memStore := session.NewMemoryStore(1*time.Hour, 10*time.Minute)
	t.Cleanup(func() { memStore.Close() })
	mgr := session.NewManager(memStore, 1*time.Hour, "Memory")

	srv := NewServer(s, mgr, nil)
	router := srv.Router()
	return router, mgr, memStore
}

// createSessionAndCookie creates a session via the manager and returns the cookie
// that should be attached to requests to authenticate as that session.
func createSessionAndCookie(t *testing.T, mgr *session.Manager, username, authMethod string) (*http.Cookie, string) {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	err := mgr.CreateWithMethod(w, r, username, authMethod, "")
	require.NoError(t, err)

	// Extract session cookie from response
	resp := w.Result()
	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == session.CookieName {
			sessionCookie = c
			break
		}
	}
	require.NotNil(t, sessionCookie, "session cookie not set")

	// Get session ID from the cookie value
	return sessionCookie, sessionCookie.Value
}

func TestHandleListSessions(t *testing.T) {
	router, mgr, _ := setupSessionTestRouter(t)

	// Create two sessions
	cookie1, id1 := createSessionAndCookie(t, mgr, "admin", "local")
	_, id2 := createSessionAndCookie(t, mgr, "user2", "oidc")

	// Request as admin (cookie1)
	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	req.AddCookie(cookie1)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var sessions []SessionInfo
	require.NoError(t, json.NewDecoder(w.Body).Decode(&sessions))
	require.Len(t, sessions, 2)

	// Find sessions by ID
	var found1, found2 *SessionInfo
	for i := range sessions {
		if sessions[i].ID == id1 {
			found1 = &sessions[i]
		}
		if sessions[i].ID == id2 {
			found2 = &sessions[i]
		}
	}
	require.NotNil(t, found1, "admin session not found")
	require.NotNil(t, found2, "user2 session not found")

	assert.True(t, found1.IsCurrent, "admin session should be marked current")
	assert.False(t, found2.IsCurrent, "user2 session should not be marked current")
	assert.Equal(t, "admin", found1.Username)
	assert.Equal(t, "local", found1.AuthMethod)
	assert.Equal(t, "user2", found2.Username)
	assert.Equal(t, "oidc", found2.AuthMethod)
	assert.NotEmpty(t, found1.CreatedAt)
	assert.NotEmpty(t, found1.LastActivity)
	assert.NotEmpty(t, found1.ExpiresIn)

	// Current session should be first (sorted)
	assert.True(t, sessions[0].IsCurrent, "current session should be first in list")
}

func TestHandleRevokeSession(t *testing.T) {
	router, mgr, _ := setupSessionTestRouter(t)

	// Create own session and a target session
	cookie1, _ := createSessionAndCookie(t, mgr, "admin", "local")
	_, targetID := createSessionAndCookie(t, mgr, "user2", "oidc")

	// Revoke target session
	req := httptest.NewRequest(http.MethodDelete, "/api/sessions/"+targetID, nil)
	req.AddCookie(cookie1)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, true, resp["ok"])

	// Verify revoked session is gone
	req2 := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	req2.AddCookie(cookie1)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	var remaining []SessionInfo
	require.NoError(t, json.NewDecoder(w2.Body).Decode(&remaining))
	assert.Len(t, remaining, 1)
	assert.Equal(t, "admin", remaining[0].Username)
}

func TestHandleRevokeOwnSession(t *testing.T) {
	router, mgr, _ := setupSessionTestRouter(t)

	cookie1, ownID := createSessionAndCookie(t, mgr, "admin", "local")

	req := httptest.NewRequest(http.MethodDelete, "/api/sessions/"+ownID, nil)
	req.AddCookie(cookie1)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code, "body: %s", w.Body.String())

	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "cannot revoke your own session")
}

func TestHandleRevokeAllOthers(t *testing.T) {
	router, mgr, _ := setupSessionTestRouter(t)

	// Create own session + 2 others
	cookie1, _ := createSessionAndCookie(t, mgr, "admin", "local")
	createSessionAndCookie(t, mgr, "user2", "oidc")
	createSessionAndCookie(t, mgr, "user3", "local")

	// Revoke all others
	req := httptest.NewRequest(http.MethodDelete, "/api/sessions", nil)
	req.AddCookie(cookie1)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, true, resp["ok"])
	assert.InDelta(t, float64(2), resp["count"], 0)

	// Verify only own session remains
	req2 := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	req2.AddCookie(cookie1)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	var remaining []SessionInfo
	require.NoError(t, json.NewDecoder(w2.Body).Decode(&remaining))
	assert.Len(t, remaining, 1)
	assert.Equal(t, "admin", remaining[0].Username)
	assert.True(t, remaining[0].IsCurrent)
}
