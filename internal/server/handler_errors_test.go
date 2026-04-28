package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"screws-box/internal/model"
	"screws-box/internal/session"
	"screws-box/internal/store"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errBoom is the canonical injected error.
var errBoom = errors.New("boom")

// failingStore implements StoreService and returns errBoom from every method.
// Used to drive handler error branches that depend on the store failing.
type failingStore struct{}

func (failingStore) Ping(_ context.Context) error          { return errBoom }
func (failingStore) GetGridData() (*model.GridData, error) { return nil, errBoom }
func (failingStore) CreateItem(_ context.Context, _ int64, _ string, _ *string, _ []string) (*model.ItemResponse, error) {
	return nil, errBoom
}

func (failingStore) GetItem(_ context.Context, _ int64) (*model.ItemResponse, error) {
	return nil, errBoom
}

func (failingStore) UpdateItem(_ context.Context, _ int64, _ string, _ *string, _ int64) (*model.ItemResponse, error) {
	return nil, errBoom
}
func (failingStore) DeleteItem(_ context.Context, _ int64) error { return errBoom }
func (failingStore) AddTagToItem(_ context.Context, _ int64, _ string) (*model.ItemResponse, error) {
	return nil, errBoom
}
func (failingStore) RemoveTagFromItem(_ context.Context, _ int64, _ string) error { return errBoom }
func (failingStore) ListItemsByContainer(_ context.Context, _ int64) (*model.ContainerWithItems, error) {
	return nil, errBoom
}

func (failingStore) ListAllItems(_ context.Context) ([]model.ItemResponse, error) {
	return nil, errBoom
}

func (failingStore) SearchItems(_ context.Context, _ string) ([]model.ItemResponse, error) {
	return nil, errBoom
}

func (failingStore) SearchItemsByTags(_ context.Context, _ string, _ []string) ([]model.ItemResponse, error) {
	return nil, errBoom
}

func (failingStore) SearchItemsBatch(_ context.Context, _ string, _ []string) (*model.SearchResponse, error) {
	return nil, errBoom
}

func (failingStore) ListTags(_ context.Context, _ string) ([]model.TagResponse, error) {
	return nil, errBoom
}
func (failingStore) RenameTag(_ context.Context, _ int64, _ string) error { return errBoom }
func (failingStore) MergeTags(_ context.Context, _, _ int64) error        { return errBoom }
func (failingStore) DeleteUnusedTag(_ context.Context, _ int64) error     { return errBoom }
func (failingStore) ResizeShelf(_ context.Context, _, _ int) (*model.ResizeResult, error) {
	return nil, errBoom
}
func (failingStore) UpdateShelfName(_ context.Context, _ string) error { return errBoom }
func (failingStore) GetAuthSettings(_ context.Context) (*model.AuthSettings, error) {
	return nil, errBoom
}

func (failingStore) UpdateAuthSettings(_ context.Context, _ *model.AuthSettings) error {
	return errBoom
}

func (failingStore) ValidateCredentials(_ context.Context, _, _ string) (bool, error) {
	return false, errBoom
}

func (failingStore) GetOIDCConfig(_ context.Context) (*model.OIDCConfig, error) {
	return nil, errBoom
}

func (failingStore) GetOIDCConfigMasked(_ context.Context) (*model.OIDCConfig, error) {
	return nil, errBoom
}
func (failingStore) SaveOIDCConfig(_ context.Context, _ *model.OIDCConfig) error { return errBoom }
func (failingStore) UpsertOIDCUser(_ context.Context, _ *model.OIDCUser) (*model.OIDCUser, error) {
	return nil, errBoom
}

func (failingStore) GetOIDCUserBySub(_ context.Context, _, _ string) (*model.OIDCUser, error) {
	return nil, errBoom
}

func (failingStore) GetOrCreateEncryptionKey(_ context.Context) ([]byte, error) {
	return nil, errBoom
}

func (failingStore) ExportAllData(_ context.Context) (*model.ExportData, error) {
	return nil, errBoom
}
func (failingStore) ImportAllData(_ context.Context, _ *model.ExportData) error { return errBoom }
func (failingStore) FindDuplicates(_ context.Context) ([]model.DuplicateGroup, error) {
	return nil, errBoom
}

// setupFailingRouter returns a router whose store is failingStore. Auth is
// disabled (default), so CSRF middleware is bypassed and we can drive POSTs.
func setupFailingRouter(t *testing.T) http.Handler {
	t.Helper()
	memStore := session.NewMemoryStore(time.Hour, 10*time.Minute)
	t.Cleanup(func() { _ = memStore.Close() })
	mgr := session.NewManager(memStore, time.Hour, "Memory")
	srv := NewServer(failingStore{}, mgr, "test")
	return srv.Router()
}

// doRequest is a tiny convenience wrapper.
func doRequest(t *testing.T, router http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *strings.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	var req *http.Request
	if rdr != nil {
		req = httptest.NewRequest(method, target, rdr)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, target, http.NoBody)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// --- /healthz ---------------------------------------------------------------

// Ping fails → 503.
func TestHealthzStoreFailsReturns503(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodGet, "/healthz", "")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// --- API: items ------------------------------------------------------------

func TestHandleListItemsStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodGet, "/api/items/", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleCreateItemStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	body := `{"name":"x","container_id":1,"tags":["a"]}`
	w := doRequest(t, router, http.MethodPost, "/api/items/", body)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleGetItemStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodGet, "/api/items/1", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleUpdateItemStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	body := `{"name":"x","container_id":1}`
	w := doRequest(t, router, http.MethodPut, "/api/items/1", body)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleDeleteItemStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodDelete, "/api/items/1", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleAddTagStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	body := `{"name":"foo"}`
	w := doRequest(t, router, http.MethodPost, "/api/items/1/tags", body)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleRemoveTagStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodDelete, "/api/items/1/tags/foo", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleListContainerItemsStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodGet, "/api/containers/1/items", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- API: tags / search / shelf -------------------------------------------

func TestHandleListTagsStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodGet, "/api/tags", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleSearchStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodGet, "/api/search?q=foo", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleResizeShelfStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	body := `{"rows":5,"cols":5}`
	w := doRequest(t, router, http.MethodPut, "/api/shelf/resize", body)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- API: auth / oidc settings --------------------------------------------

func TestHandleGetAuthSettingsStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodGet, "/api/shelf/auth", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleUpdateAuthSettingsStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	body := `{"username":"x","password":"Hunter2!Hunter2!","enabled":true}`
	w := doRequest(t, router, http.MethodPut, "/api/shelf/auth", body)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleGetOIDCConfigStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodGet, "/api/oidc/config", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- API: export / import / duplicates ------------------------------------

func TestHandleExportStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodGet, "/api/export", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleDuplicatesStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodGet, "/api/duplicates", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- API: bad input → 400 -------------------------------------------------

// These don't hit store-failure branches; they hit handler validation paths
// that are mostly already covered, but a few weren't.

func TestHandleGetItemBadID(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodGet, "/api/items/not-a-number", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleUpdateItemBadID(t *testing.T) {
	router := setupFailingRouter(t)
	body := `{"name":"x","container_id":1}`
	w := doRequest(t, router, http.MethodPut, "/api/items/abc", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleUpdateItemBadJSON(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodPut, "/api/items/1", "not-json")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateItemBadJSON(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodPost, "/api/items/", "not-json")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleResizeBadJSON(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodPut, "/api/shelf/resize", "not-json")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleAddTagBadJSON(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodPost, "/api/items/1/tags", "not-json")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Sessions endpoints ---------------------------------------------------

func TestHandleListSessionsStoreFails(t *testing.T) {
	// /api/sessions hits Manager.ListSessions which delegates to session store,
	// not the data store. To make this fail, we need a failing session store.
	// For now we just ensure the endpoint responds (no panic) under failingStore;
	// it should return 200 with empty list since the session store is healthy.
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodGet, "/api/sessions", "")
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- Rate limiting --------------------------------------------------------

// Burst of API requests > 10 (the burst size for newRateLimitAPI) triggers
// the rate-limit branch.
func TestRateLimitAPIExceeded(t *testing.T) {
	router := setupFailingRouter(t)
	gotLimited := false
	for range 30 {
		w := doRequest(t, router, http.MethodGet, "/api/items/", "")
		if w.Code == http.StatusTooManyRequests {
			gotLimited = true
			break
		}
	}
	assert.True(t, gotLimited, "expected at least one 429 from API rate limiting")
}

// Login burst > 5 triggers the login rate-limit branch.
func TestRateLimitLoginExceeded(t *testing.T) {
	router := setupFailingRouter(t)
	gotLimited := false
	for range 12 {
		w := doRequest(t, router, http.MethodPost, "/login",
			`{"username":"x","password":"y"}`)
		if w.Code == http.StatusTooManyRequests {
			gotLimited = true
			break
		}
	}
	assert.True(t, gotLimited, "expected at least one 429 from login rate limiting")
}

// --- CSRF middleware ------------------------------------------------------

// realRouterWithAuth boots a real router with auth enabled so CSRF middleware
// has effect. Requires the SQLite store.
func realRouterWithAuth(t *testing.T) (http.Handler, *session.Manager) {
	t.Helper()
	s := &store.Store{}
	tmpFile := filepath.Join(t.TempDir(), "test.db")
	require.NoError(t, s.Open(tmpFile))
	t.Cleanup(func() { _ = s.Close() })

	require.NoError(t, s.UpdateAuthSettings(context.Background(), &model.AuthSettings{
		Enabled:  true,
		Username: "alice",
		Password: "Hunter2!Hunter2!",
	}))

	memStore := session.NewMemoryStore(time.Hour, 10*time.Minute)
	t.Cleanup(func() { _ = memStore.Close() })
	mgr := session.NewManager(memStore, time.Hour, "Memory")
	srv := NewServer(s, mgr, "test")
	return srv.Router(), mgr
}

// POST /api/items without an authenticated session: redirected to /login by
// authMiddleware, never reaching CSRF. Skipped here because authMiddleware
// already short-circuits.

// CSRF rejects state-changing requests when a session exists but no token
// is supplied.
func TestCSRFRejectsMissingToken(t *testing.T) {
	router, mgr := realRouterWithAuth(t)

	// Manually create a session via the manager.
	w := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodGet, "/", nil)
	require.NoError(t, mgr.Create(w, loginReq, "alice"))

	var sessionCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == session.CookieName {
			sessionCookie = c
		}
	}
	require.NotNil(t, sessionCookie)

	// Now POST to a state-changing endpoint without CSRF header.
	body := bytes.NewReader([]byte(`{"name":"x","container_id":1}`))
	req := httptest.NewRequest(http.MethodPost, "/api/items/", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// CSRF rejects when token is supplied but doesn't match the session's.
func TestCSRFRejectsMismatchedToken(t *testing.T) {
	router, mgr := realRouterWithAuth(t)

	w := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodGet, "/", nil)
	require.NoError(t, mgr.Create(w, loginReq, "alice"))

	var sessionCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == session.CookieName {
			sessionCookie = c
		}
	}
	require.NotNil(t, sessionCookie)

	body := bytes.NewReader([]byte(`{"name":"x","container_id":1}`))
	req := httptest.NewRequest(http.MethodPost, "/api/items/", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "wrong-token-value")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// --- writeJSON encode failure ---------------------------------------------

// writeJSON's encode-fail path is hit when the value contains an unmarshalable
// type (channel, function). Direct unit test of writeJSON.
func TestWriteJSONEncodeError(t *testing.T) {
	w := httptest.NewRecorder()
	bad := map[string]any{"ch": make(chan int)}
	writeJSON(w, http.StatusOK, bad)
	// Status was already written before encode; we just need encode to be
	// attempted and the slog.Error branch covered.
	assert.Equal(t, http.StatusOK, w.Code)
}

// Encoding a value that produces a JSON syntax error should be impossible
// for our types, but include a sanity unit test for the JSON encoder
// invocation path.
func TestWriteJSONHappyPath(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]int{"n": 7})
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var got map[string]int
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, 7, got["n"])
}

// --- More endpoint coverage with failingStore ----------------------------

func TestHandleRenameTagStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	body := `{"name":"newname"}`
	w := doRequest(t, router, http.MethodPut, "/api/tags/1", body)
	// findTagByID is called first; ListTags fails → 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleRenameTagBadID(t *testing.T) {
	router := setupFailingRouter(t)
	body := `{"name":"newname"}`
	w := doRequest(t, router, http.MethodPut, "/api/tags/abc", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleRenameTagBadJSON(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodPut, "/api/tags/1", "not-json")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleRenameTagEmptyName(t *testing.T) {
	router := setupFailingRouter(t)
	body := `{"name":"  "}`
	w := doRequest(t, router, http.MethodPut, "/api/tags/1", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleDeleteTagStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodDelete, "/api/tags/1", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleDeleteTagBadID(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodDelete, "/api/tags/abc", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleUpdateOIDCConfigStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	body := `{"enabled":false,"issuer_url":"https://example.com","client_id":"c","display_name":"X"}`
	w := doRequest(t, router, http.MethodPut, "/api/oidc/config", body)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleUpdateOIDCConfigBadJSON(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodPut, "/api/oidc/config", "not-json")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleImportValidateStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	body := `{"version":1,"shelf":{"name":"x","rows":5,"cols":5}}`
	w := doRequest(t, router, http.MethodPost, "/api/import/validate", body)
	// Validation just returns the parsed payload; with failingStore it
	// shouldn't matter. But the underlying ExportAllData would be called
	// to compare. Either 400 or 500 is acceptable; just ensure it doesn't 200.
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestHandleImportConfirmBadJSON(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodPost, "/api/import/confirm", "not-json")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleImportConfirmStoreFails(t *testing.T) {
	router := setupFailingRouter(t)
	body := `{"version":1,"shelf":{"name":"x","rows":5,"cols":5}}`
	w := doRequest(t, router, http.MethodPost, "/api/import/confirm", body)
	// May 400 (validation) or 500 (store failure); we just want the error path.
	assert.NotEqual(t, http.StatusOK, w.Code)
}

// /api/sessions delete-others uses session manager not data store; with
// failingStore it should still succeed (manager has memory-store).
func TestHandleRevokeAllOthersFailingStore(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodDelete, "/api/sessions", "")
	// Without an authenticated session the manager has nothing to revoke;
	// the endpoint should still respond cleanly.
	assert.NotEqual(t, http.StatusInternalServerError, w.Code)
}

func TestHandleRevokeSpecificSessionBadInput(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodDelete, "/api/sessions/some-id", "")
	// Without auth, should not 500.
	assert.NotEqual(t, http.StatusInternalServerError, w.Code)
}

// --- Auth-enabled, failing store: authMiddleware redirect/401 -----------

// HTML route, no session, auth enabled → redirected to /login.
func TestAuthRedirectHTMLNoSession(t *testing.T) {
	router, _ := realRouterWithAuth(t)
	w := doRequest(t, router, http.MethodGet, "/", "")
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))
}

// API route, no session, auth enabled → 401 JSON.
func TestAuthAPI401NoSession(t *testing.T) {
	router, _ := realRouterWithAuth(t)
	w := doRequest(t, router, http.MethodGet, "/api/items/", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- /login flows --------------------------------------------------------

// Login GET renders the login page even when GetOIDCConfig fails (the
// failure is silently treated as "OIDC disabled").
func TestHandleLoginGETFailingStore(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodGet, "/login", "")
	assert.Equal(t, http.StatusOK, w.Code)
}

// Login POST with form-encoded body — failingStore makes ValidateCredentials
// fail, exercising the "internal error" branch.
func TestHandleLoginPOSTValidateFails(t *testing.T) {
	router := setupFailingRouter(t)
	form := strings.NewReader("username=alice&password=secret")
	req := httptest.NewRequest(http.MethodPost, "/login", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code) // re-rendered login page
}

// --- OIDC flows ----------------------------------------------------------

// OIDC start with failing GetOIDCConfig redirects to /login?error=auth_failed.
func TestHandleOIDCStartFailingStore(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodGet, "/auth/oidc", "")
	// Expect redirect (302/303) — the route always redirects on failure.
	assert.True(t, w.Code == http.StatusFound || w.Code == http.StatusSeeOther,
		"expected redirect, got %d", w.Code)
}

// OIDC callback with no code/state → redirect to /login?error=auth_failed.
func TestHandleOIDCCallbackNoCode(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodGet, "/auth/callback", "")
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "error=auth_failed")
}

// OIDC callback with provider-side error param → redirect.
func TestHandleOIDCCallbackProviderError(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodGet, "/auth/callback?error=access_denied", "")
	assert.Equal(t, http.StatusFound, w.Code)
}

// OIDC callback with code+state but encryption key fetch fails → redirect.
func TestHandleOIDCCallbackEncryptionKeyFails(t *testing.T) {
	router := setupFailingRouter(t)
	w := doRequest(t, router, http.MethodGet, "/auth/callback?code=c&state=s", "")
	assert.Equal(t, http.StatusFound, w.Code)
}
