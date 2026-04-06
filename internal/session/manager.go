package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
)

const (
	// CookieName is the session cookie name.
	CookieName = "screwsbox_session"
	// CSRFCookieName is the CSRF double-submit cookie name.
	CSRFCookieName = "screwsbox_csrf"
)

// Manager wraps a Store and handles cookie read/write for sessions.
// Handlers call Manager methods instead of raw cookie operations.
type Manager struct {
	store     Store
	ttl       time.Duration
	storeType string
}

// NewManager creates a Manager with the given Store, TTL, and store type label.
func NewManager(store Store, ttl time.Duration, storeType string) *Manager {
	return &Manager{store: store, ttl: ttl, storeType: storeType}
}

// generateToken returns a cryptographically random 64-char hex string (256 bits).
func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// isSecure returns true if the request arrived over HTTPS.
func isSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return r.Header.Get("X-Forwarded-Proto") == "https"
}

// Create generates a new session for the given username, stores it,
// and sets the session + CSRF cookies on the response.
// AuthMethod is set to "local" for backward compatibility.
func (m *Manager) Create(w http.ResponseWriter, r *http.Request, username string) error {
	return m.CreateWithMethod(w, r, username, "local", "")
}

// CreateWithMethod generates a new session with the specified auth method,
// stores it, and sets the session + CSRF cookies on the response.
func (m *Manager) CreateWithMethod(w http.ResponseWriter, r *http.Request, username, authMethod, displayName string) error {
	sess := &Session{
		ID:           generateToken(),
		Username:     username,
		AuthMethod:   authMethod,
		DisplayName:  displayName,
		CSRFToken:    generateToken(),
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}
	if err := m.store.Create(r.Context(), sess); err != nil {
		return err
	}
	secure := isSecure(r)
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    sess.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    sess.CSRFToken,
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// Destroy deletes the session and clears cookies.
// Clears both Secure=true and Secure=false variants to handle
// stale cookies from HTTPS deployments.
func (m *Manager) Destroy(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(CookieName)
	if err == nil {
		m.store.Delete(r.Context(), c.Value)
	}
	for _, sec := range []bool{false, true} {
		http.SetCookie(w, &http.Cookie{
			Name: CookieName, Value: "", Path: "/",
			MaxAge: -1, HttpOnly: true, Secure: sec,
		})
		http.SetCookie(w, &http.Cookie{
			Name: CSRFCookieName, Value: "", Path: "/",
			MaxAge: -1, Secure: sec,
		})
	}
}

// GetUser returns the username for the current session, or "" if no valid session.
// Also calls Touch for sliding window expiry.
func (m *Manager) GetUser(r *http.Request) string {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	sess, err := m.store.Get(r.Context(), c.Value)
	if err != nil || sess == nil {
		return ""
	}
	m.store.Touch(r.Context(), c.Value)
	return sess.Username
}

// GetSession returns the full Session for the current request, or nil if not authenticated.
func (m *Manager) GetSession(r *http.Request) *Session {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return nil
	}
	sess, err := m.store.Get(r.Context(), c.Value)
	if err != nil || sess == nil {
		return nil
	}
	m.store.Touch(r.Context(), c.Value)
	return sess
}

// DeleteByAuthMethod deletes all sessions with the given auth method.
// Returns the number of sessions deleted.
func (m *Manager) DeleteByAuthMethod(ctx context.Context, method string) (int, error) {
	return m.store.DeleteByAuthMethod(ctx, method)
}

// GetCSRFToken returns the server-side CSRF token for the current session.
func (m *Manager) GetCSRFToken(r *http.Request) string {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	sess, err := m.store.Get(r.Context(), c.Value)
	if err != nil || sess == nil {
		return ""
	}
	return sess.CSRFToken
}

// ListSessions returns all active sessions from the store.
func (m *Manager) ListSessions(ctx context.Context) ([]*Session, error) {
	return m.store.List(ctx)
}

// DeleteSession deletes a session by ID from the store.
// Used by admin revocation.
func (m *Manager) DeleteSession(ctx context.Context, id string) error {
	return m.store.Delete(ctx, id)
}

// StoreType returns "Redis" or "Memory" for display in admin UI.
func (m *Manager) StoreType() string {
	return m.storeType
}

// TTL returns the session TTL duration for expiry calculations.
func (m *Manager) TTL() time.Duration {
	return m.ttl
}
