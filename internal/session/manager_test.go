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
	mgr := NewManager(store, ttl)
	return mgr, store
}

func TestManager_Create(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(store.Close)

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
	t.Cleanup(store.Close)

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
	t.Cleanup(store.Close)

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
	t.Cleanup(store.Close)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	user := mgr.GetUser(r)
	assert.Empty(t, user)
}

func TestManager_GetUser_ExpiredSession(t *testing.T) {
	mgr, store := newTestManager(50 * time.Millisecond)
	t.Cleanup(store.Close)

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
	t.Cleanup(store.Close)

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
	t.Cleanup(store.Close)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	token := mgr.GetCSRFToken(r)
	assert.Empty(t, token)
}

func TestManager_GetUser_TouchesSession(t *testing.T) {
	mgr, store := newTestManager(150 * time.Millisecond)
	t.Cleanup(store.Close)

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
