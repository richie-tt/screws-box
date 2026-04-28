package session

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingStore implements the Store interface but every method returns errStoreFailure.
// Used to drive Manager error branches that depend on the underlying store failing.

var errStoreFailure = errors.New("injected store failure")

type failingStore struct{}

func (failingStore) Create(_ context.Context, _ *Session) error        { return errStoreFailure }
func (failingStore) Get(_ context.Context, _ string) (*Session, error) { return nil, errStoreFailure }
func (failingStore) Delete(_ context.Context, _ string) error          { return errStoreFailure }
func (failingStore) Touch(_ context.Context, _ string) error           { return errStoreFailure }
func (failingStore) DeleteByAuthMethod(_ context.Context, _ string) (int, error) {
	return 0, errStoreFailure
}
func (failingStore) List(_ context.Context) ([]*Session, error) { return nil, errStoreFailure }
func (failingStore) Close() error                               { return nil }

// isSecure returns true when r.TLS != nil. Build a request with a
// non-nil ConnectionState to hit the TLS branch (manager.go:40-42).
func TestIsSecureTLSConnection(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.TLS = &tls.ConnectionState{} // simulate HTTPS request
	assert.True(t, isSecure(r))
}

// CreateWithMethod returns the store error when the store fails — covers
// the err-propagation branch at manager.go:65-67.
func TestManagerCreateStoreFailurePropagates(t *testing.T) {
	mgr := NewManager(failingStore{}, time.Hour, "Failing")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/login", nil)
	err := mgr.Create(w, r, "alice")
	require.Error(t, err)
	assert.ErrorIs(t, err, errStoreFailure)
}

// GetCSRFToken returns "" when the cookie is present but the store
// reports an error (or returns nil). Covers manager.go:150-152.
func TestManagerGetCSRFTokenStoreFails(t *testing.T) {
	mgr := NewManager(failingStore{}, time.Hour, "Failing")
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: CookieName, Value: "stale-id"})

	token := mgr.GetCSRFToken(r)
	assert.Empty(t, token)
}

// GetCSRFToken returns "" when the cookie is present but the store
// reports session not found (nil sess). Combines with the err-or-nil
// short-circuit in the same branch.
func TestManagerGetCSRFTokenSessionMissing(t *testing.T) {
	mgr, store := newTestManager(time.Hour)
	t.Cleanup(func() { _ = store.Close() })

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: CookieName, Value: "no-such-id"})

	token := mgr.GetCSRFToken(r)
	assert.Empty(t, token)
}
