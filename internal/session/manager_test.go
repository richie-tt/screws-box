package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestManager(ttl time.Duration) (*Manager, *MemoryStore) {
	store := NewMemoryStore(ttl, 5*time.Minute)
	mgr := NewManager(store, ttl, "Memory")
	return mgr, store
}

func TestManager_Create(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/login", nil)

	err := mgr.Create(w, r, "alice")
	require.NoError(t, err)

	resp := w.Result()
	cookies := resp.Cookies()

	// Should set two cookies: session + CSRF
	var sessionCookie, csrfCookie *http.Cookie
	for _, c := range cookies {
		switch c.Name {
		case CookieName:
			sessionCookie = c
		case CSRFCookieName:
			csrfCookie = c
		}
	}

	require.NotNil(t, sessionCookie, "session cookie should be set")
	require.NotNil(t, csrfCookie, "CSRF cookie should be set")

	assert.Len(t, sessionCookie.Value, 64, "session token should be 64 hex chars")
	assert.Len(t, csrfCookie.Value, 64, "CSRF token should be 64 hex chars")
	assert.True(t, sessionCookie.HttpOnly, "session cookie should be HttpOnly")
	assert.False(t, csrfCookie.HttpOnly, "CSRF cookie should NOT be HttpOnly")
	assert.Equal(t, "/", sessionCookie.Path)
	assert.Equal(t, http.SameSiteLaxMode, sessionCookie.SameSite)
}

func TestManager_Destroy(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	// Create a session first
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/login", nil)
	require.NoError(t, mgr.Create(w, r, "alice"))

	sessionCookieValue := ""
	for _, c := range w.Result().Cookies() {
		if c.Name == CookieName {
			sessionCookieValue = c.Value
		}
	}
	require.NotEmpty(t, sessionCookieValue)

	// Now destroy
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodPost, "/logout", nil)
	r2.AddCookie(&http.Cookie{Name: CookieName, Value: sessionCookieValue})
	mgr.Destroy(w2, r2)

	// Should clear cookies with both Secure=true and Secure=false (4 Set-Cookie headers)
	setCookies := w2.Result().Cookies()
	clearCount := 0
	for _, c := range setCookies {
		if (c.Name == CookieName || c.Name == CSRFCookieName) && c.MaxAge < 0 {
			clearCount++
		}
	}
	assert.Equal(t, 4, clearCount, "should clear 4 cookies (session+csrf x secure+insecure)")

	// Session should be gone from store
	user := mgr.GetUser(r2)
	assert.Empty(t, user)
}

func TestManager_GetUser(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	// Create session
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/login", nil)
	require.NoError(t, mgr.Create(w, r, "alice"))

	sessionCookieValue := ""
	for _, c := range w.Result().Cookies() {
		if c.Name == CookieName {
			sessionCookieValue = c.Value
		}
	}

	// Get user with cookie
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.AddCookie(&http.Cookie{Name: CookieName, Value: sessionCookieValue})
	user := mgr.GetUser(r2)
	assert.Equal(t, "alice", user)
}

func TestManager_GetUser_NoSession(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	user := mgr.GetUser(r)
	assert.Empty(t, user)
}

func TestManager_GetUser_ExpiredSession(t *testing.T) {
	mgr, store := newTestManager(50 * time.Millisecond)
	t.Cleanup(func() { store.Close() })

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/login", nil)
	require.NoError(t, mgr.Create(w, r, "alice"))

	sessionCookieValue := ""
	for _, c := range w.Result().Cookies() {
		if c.Name == CookieName {
			sessionCookieValue = c.Value
		}
	}

	time.Sleep(100 * time.Millisecond)

	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.AddCookie(&http.Cookie{Name: CookieName, Value: sessionCookieValue})
	user := mgr.GetUser(r2)
	assert.Empty(t, user, "expired session should return empty username")
}

func TestManager_GetCSRFToken(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/login", nil)
	require.NoError(t, mgr.Create(w, r, "alice"))

	sessionCookieValue := ""
	csrfCookieValue := ""
	for _, c := range w.Result().Cookies() {
		switch c.Name {
		case CookieName:
			sessionCookieValue = c.Value
		case CSRFCookieName:
			csrfCookieValue = c.Value
		}
	}

	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.AddCookie(&http.Cookie{Name: CookieName, Value: sessionCookieValue})
	token := mgr.GetCSRFToken(r2)
	assert.Equal(t, csrfCookieValue, token, "server-side CSRF token should match cookie value")
}

func TestManager_GetCSRFToken_NoSession(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	token := mgr.GetCSRFToken(r)
	assert.Empty(t, token)
}

func TestManager_CreateSetsLocalAuthMethod(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/login", nil)

	err := mgr.Create(w, r, "alice")
	require.NoError(t, err)

	// Find the session in the store
	sessionCookieValue := ""
	for _, c := range w.Result().Cookies() {
		if c.Name == CookieName {
			sessionCookieValue = c.Value
		}
	}
	require.NotEmpty(t, sessionCookieValue)

	sess, err := store.Get(r.Context(), sessionCookieValue)
	require.NoError(t, err)
	require.NotNil(t, sess)
	assert.Equal(t, "local", sess.AuthMethod, "Create should set AuthMethod to 'local'")
}

func TestManager_CreateWithMethod_OIDC(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/callback", nil)

	err := mgr.CreateWithMethod(w, r, "alice", "oidc", "")
	require.NoError(t, err)

	sessionCookieValue := ""
	for _, c := range w.Result().Cookies() {
		if c.Name == CookieName {
			sessionCookieValue = c.Value
		}
	}
	require.NotEmpty(t, sessionCookieValue)

	sess, err := store.Get(r.Context(), sessionCookieValue)
	require.NoError(t, err)
	require.NotNil(t, sess)
	assert.Equal(t, "oidc", sess.AuthMethod, "CreateWithMethod should set AuthMethod to 'oidc'")
}

func TestManager_CreateWithMethod_SetsCookies(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/callback", nil)

	err := mgr.CreateWithMethod(w, r, "alice", "oidc", "")
	require.NoError(t, err)

	resp := w.Result()
	cookies := resp.Cookies()

	var sessionCookie, csrfCookie *http.Cookie
	for _, c := range cookies {
		switch c.Name {
		case CookieName:
			sessionCookie = c
		case CSRFCookieName:
			csrfCookie = c
		}
	}

	require.NotNil(t, sessionCookie, "session cookie should be set")
	require.NotNil(t, csrfCookie, "CSRF cookie should be set")
	assert.Len(t, sessionCookie.Value, 64, "session token should be 64 hex chars")
	assert.Len(t, csrfCookie.Value, 64, "CSRF token should be 64 hex chars")
	assert.True(t, sessionCookie.HttpOnly, "session cookie should be HttpOnly")
	assert.False(t, csrfCookie.HttpOnly, "CSRF cookie should NOT be HttpOnly")
}

func TestManager_GetSession(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	// Create a session with CreateWithMethod so we can verify all fields
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/callback", nil)
	require.NoError(t, mgr.CreateWithMethod(w, r, "alice", "oidc", "Alice Smith"))

	sessionCookieValue := ""
	for _, c := range w.Result().Cookies() {
		if c.Name == CookieName {
			sessionCookieValue = c.Value
		}
	}
	require.NotEmpty(t, sessionCookieValue)

	// Retrieve the full session via GetSession
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.AddCookie(&http.Cookie{Name: CookieName, Value: sessionCookieValue})
	sess := mgr.GetSession(r2)

	require.NotNil(t, sess, "GetSession should return a non-nil session")
	assert.Equal(t, sessionCookieValue, sess.ID)
	assert.Equal(t, "alice", sess.Username)
	assert.Equal(t, "oidc", sess.AuthMethod)
	assert.Equal(t, "Alice Smith", sess.DisplayName)
	assert.NotEmpty(t, sess.CSRFToken)
	assert.False(t, sess.CreatedAt.IsZero())
	assert.False(t, sess.LastActivity.IsZero())
}

func TestManager_GetSession_NoSession(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	sess := mgr.GetSession(r)
	assert.Nil(t, sess, "GetSession with no cookie should return nil")
}

func TestManager_GetSession_InvalidCookie(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: CookieName, Value: "nonexistent_token"})
	sess := mgr.GetSession(r)
	assert.Nil(t, sess, "GetSession with invalid cookie should return nil")
}

func TestManager_GetSession_ExpiredSession(t *testing.T) {
	mgr, store := newTestManager(50 * time.Millisecond)
	t.Cleanup(func() { store.Close() })

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/login", nil)
	require.NoError(t, mgr.Create(w, r, "alice"))

	sessionCookieValue := ""
	for _, c := range w.Result().Cookies() {
		if c.Name == CookieName {
			sessionCookieValue = c.Value
		}
	}

	time.Sleep(100 * time.Millisecond)

	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.AddCookie(&http.Cookie{Name: CookieName, Value: sessionCookieValue})
	sess := mgr.GetSession(r2)
	assert.Nil(t, sess, "GetSession for expired session should return nil")
}

func TestManager_DeleteByAuthMethod(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	// Create two OIDC sessions and one local session
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest(http.MethodPost, "/auth/callback", nil)
	require.NoError(t, mgr.CreateWithMethod(w1, r1, "alice", "oidc", ""))

	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodPost, "/auth/callback", nil)
	require.NoError(t, mgr.CreateWithMethod(w2, r2, "bob", "oidc", ""))

	w3 := httptest.NewRecorder()
	r3 := httptest.NewRequest(http.MethodPost, "/login", nil)
	require.NoError(t, mgr.Create(w3, r3, "carol"))

	// Delete all OIDC sessions
	ctx := r1.Context()
	count, err := mgr.DeleteByAuthMethod(ctx, "oidc")
	require.NoError(t, err)
	assert.Equal(t, 2, count, "should delete exactly 2 OIDC sessions")

	// Verify the local session is still present
	sessions, err := mgr.ListSessions(ctx)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "carol", sessions[0].Username)
	assert.Equal(t, "local", sessions[0].AuthMethod)
}

func TestManager_DeleteByAuthMethod_NoneMatch(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/login", nil)
	require.NoError(t, mgr.Create(w, r, "alice"))

	ctx := r.Context()
	count, err := mgr.DeleteByAuthMethod(ctx, "oidc")
	require.NoError(t, err)
	assert.Equal(t, 0, count, "should delete 0 sessions when none match")
}

func TestManager_ListSessions(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()

	// Empty store should return empty list
	sessions, err := mgr.ListSessions(ctx)
	require.NoError(t, err)
	assert.Empty(t, sessions)

	// Create two sessions
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest(http.MethodPost, "/login", nil)
	require.NoError(t, mgr.Create(w1, r1, "alice"))

	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodPost, "/login", nil)
	require.NoError(t, mgr.CreateWithMethod(w2, r2, "bob", "oidc", "Bob Jones"))

	sessions, err = mgr.ListSessions(ctx)
	require.NoError(t, err)
	assert.Len(t, sessions, 2)

	// Collect usernames to verify both are present (order not guaranteed)
	usernames := map[string]bool{}
	for _, s := range sessions {
		usernames[s.Username] = true
	}
	assert.True(t, usernames["alice"], "alice should be in session list")
	assert.True(t, usernames["bob"], "bob should be in session list")
}

func TestManager_DeleteSession(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	// Create a session
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/login", nil)
	require.NoError(t, mgr.Create(w, r, "alice"))

	sessionCookieValue := ""
	for _, c := range w.Result().Cookies() {
		if c.Name == CookieName {
			sessionCookieValue = c.Value
		}
	}
	require.NotEmpty(t, sessionCookieValue)

	ctx := r.Context()

	// Verify session exists
	sessions, err := mgr.ListSessions(ctx)
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	// Delete it by ID
	err = mgr.DeleteSession(ctx, sessionCookieValue)
	require.NoError(t, err)

	// Verify session is gone
	sessions, err = mgr.ListSessions(ctx)
	require.NoError(t, err)
	assert.Empty(t, sessions, "session list should be empty after deletion")

	// GetUser should also return empty
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.AddCookie(&http.Cookie{Name: CookieName, Value: sessionCookieValue})
	assert.Empty(t, mgr.GetUser(r2))
}

func TestManager_DeleteSession_NonexistentID(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()

	// Deleting a nonexistent session should not error (MemoryStore behavior)
	err := mgr.DeleteSession(ctx, "does_not_exist")
	assert.NoError(t, err)
}

func TestManager_StoreType(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { store.Close() })

	assert.Equal(t, "Memory", mgr.StoreType())

	// Verify a different store type label is returned when configured differently
	mgr2 := NewManager(store, time.Hour, "Redis")
	assert.Equal(t, "Redis", mgr2.StoreType())
}

func TestManager_TTL(t *testing.T) {
	ttl := 2 * time.Hour
	mgr, store := newTestManager(ttl)
	t.Cleanup(func() { store.Close() })

	assert.Equal(t, ttl, mgr.TTL())
}

func TestManager_TTL_DifferentValues(t *testing.T) {
	store := NewMemoryStore(30*time.Minute, 5*time.Minute)
	t.Cleanup(func() { store.Close() })

	mgr := NewManager(store, 30*time.Minute, "Memory")
	assert.Equal(t, 30*time.Minute, mgr.TTL())
}

func TestManager_GetUser_TouchesSession(t *testing.T) {
	mgr, store := newTestManager(150 * time.Millisecond)
	t.Cleanup(func() { store.Close() })

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/login", nil)
	require.NoError(t, mgr.Create(w, r, "alice"))

	sessionCookieValue := ""
	for _, c := range w.Result().Cookies() {
		if c.Name == CookieName {
			sessionCookieValue = c.Value
		}
	}

	// Wait 100ms, then GetUser (which should Touch)
	time.Sleep(100 * time.Millisecond)
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.AddCookie(&http.Cookie{Name: CookieName, Value: sessionCookieValue})
	user := mgr.GetUser(r2)
	assert.Equal(t, "alice", user)

	// Wait another 100ms — total 200ms from creation but only 100ms from Touch
	time.Sleep(100 * time.Millisecond)
	r3 := httptest.NewRequest(http.MethodGet, "/", nil)
	r3.AddCookie(&http.Cookie{Name: CookieName, Value: sessionCookieValue})
	user2 := mgr.GetUser(r3)
	assert.Equal(t, "alice", user2, "session should still be alive due to Touch from previous GetUser")
}
