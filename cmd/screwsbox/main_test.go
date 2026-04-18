package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"screws-box/internal/model"
	"screws-box/internal/server"
	"screws-box/internal/session"
	"screws-box/internal/store"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s := &store.Store{}
	tmpFile := filepath.Join(t.TempDir(), "test.db")
	require.NoError(t, s.Open(tmpFile))
	t.Cleanup(func() { s.Close() })
	return s
}

func testRouter(t *testing.T, s *store.Store) http.Handler {
	t.Helper()
	ms := session.NewMemoryStore(1*time.Hour, 10*time.Minute)
	t.Cleanup(func() { ms.Close() })
	mgr := session.NewManager(ms, 1*time.Hour, "Memory")
	srv := server.NewServer(s, mgr, "test")
	return srv.Router()
}

func TestGridHandler(t *testing.T) {
	s := openTestStore(t)
	router := testRouter(t, s)
	ts := httptest.NewServer(router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	require.NoError(t, err, "GET / failed")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	ct := resp.Header.Get("Content-Type")
	assert.True(t, strings.HasPrefix(ct, "text/html"), "Content-Type = %q, want text/html", ct)

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "failed to read body")
	html := string(bodyBytes)

	assert.Contains(t, html, "Screws Box")
	assert.Contains(t, html, "grid-container")
	assert.Contains(t, html, "1A")
	assert.Contains(t, html, "10E")
	assert.Contains(t, html, "cell-coord")
	assert.Contains(t, html, "cell-count")
	assert.NotContains(t, html, "Setup in progress")
}

func TestStaticCSS(t *testing.T) {
	s := openTestStore(t)
	router := testRouter(t, s)
	ts := httptest.NewServer(router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/static/css/app.css")
	require.NoError(t, err, "GET static CSS failed")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServerBindAddress(t *testing.T) {
	s := openTestStore(t)

	listener, err := net.Listen("tcp", "0.0.0.0:0") //nolint:gosec // G102: test server, binding all interfaces is intentional
	require.NoError(t, err, "failed to listen")
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	srv := &http.Server{ //nolint:gosec // test server, Slowloris not a concern
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: testRouter(t, s),
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Errorf("server error: %v", err)
		}
	}()
	defer srv.Close()

	time.Sleep(50 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	require.NoError(t, err, "could not reach server on port %d", port)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGracefulShutdown(t *testing.T) {
	s := openTestStore(t)

	srv := &http.Server{ //nolint:gosec // test server, Slowloris not a concern
		Addr:    "0.0.0.0:0",
		Handler: testRouter(t, s),
	}

	listener, err := net.Listen("tcp", "0.0.0.0:0") //nolint:gosec // G102: test server, binding all interfaces is intentional
	require.NoError(t, err, "failed to listen")

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(listener)
	}()

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, srv.Shutdown(ctx), "shutdown failed")

	serveErr := <-errCh
	assert.Equal(t, http.ErrServerClosed, serveErr)
}

func TestParseSessionTTL(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want time.Duration
	}{
		{name: "empty env returns default 24h", env: "", want: 24 * time.Hour},
		{name: "valid duration 1h", env: "1h", want: 1 * time.Hour},
		{name: "valid duration 30m", env: "30m", want: 30 * time.Minute},
		{name: "valid duration 2h30m", env: "2h30m", want: 2*time.Hour + 30*time.Minute},
		{name: "invalid string returns default", env: "notaduration", want: 24 * time.Hour},
		{name: "zero returns default", env: "0s", want: 24 * time.Hour},
		{name: "negative returns default", env: "-5m", want: 24 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv("SESSION_TTL", tt.env)
			} else {
				t.Setenv("SESSION_TTL", "")
			}
			got := parseSessionTTL()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSanitizeLogValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "no special chars", input: "hello world", want: "hello world"},
		{name: "strips newline", input: "hello\nworld", want: "helloworld"},
		{name: "strips carriage return", input: "hello\rworld", want: "helloworld"},
		{name: "replaces tab with space", input: "hello\tworld", want: "hello world"},
		{name: "strips mixed control chars", input: "a\nb\rc\td", want: "abc d"},
		{name: "empty string", input: "", want: ""},
		{name: "only control chars", input: "\n\r\t", want: " "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeLogValue(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSeedOIDCFromEnv(t *testing.T) {
	t.Run("skips when OIDC_ISSUER is empty", func(t *testing.T) {
		s := openTestStore(t)
		t.Setenv("OIDC_ISSUER", "")

		seedOIDCFromEnv(s)

		cfg, err := s.GetOIDCConfig(context.Background())
		require.NoError(t, err)
		assert.Empty(t, cfg.IssuerURL)
	})

	t.Run("seeds config when env vars set and DB empty", func(t *testing.T) {
		s := openTestStore(t)
		t.Setenv("OIDC_ISSUER", "https://accounts.example.com")
		t.Setenv("OIDC_CLIENT_ID", "my-client-id")
		t.Setenv("OIDC_CLIENT_SECRET", "my-secret")
		t.Setenv("OIDC_DISPLAY_NAME", "Example SSO")

		seedOIDCFromEnv(s)

		cfg, err := s.GetOIDCConfig(context.Background())
		require.NoError(t, err)
		assert.True(t, cfg.Enabled)
		assert.Equal(t, "https://accounts.example.com", cfg.IssuerURL)
		assert.Equal(t, "my-client-id", cfg.ClientID)
		assert.Equal(t, "my-secret", cfg.ClientSecret)
		assert.Equal(t, "Example SSO", cfg.DisplayName)
	})

	t.Run("skips seed when config already in DB", func(t *testing.T) {
		s := openTestStore(t)
		ctx := context.Background()

		// Pre-populate OIDC config in the DB
		err := s.SaveOIDCConfig(ctx, &model.OIDCConfig{
			Enabled:      true,
			IssuerURL:    "https://existing.example.com",
			ClientID:     "existing-client",
			ClientSecret: "existing-secret",
			DisplayName:  "Existing SSO",
		})
		require.NoError(t, err)

		// Set different env vars
		t.Setenv("OIDC_ISSUER", "https://new.example.com")
		t.Setenv("OIDC_CLIENT_ID", "new-client")
		t.Setenv("OIDC_CLIENT_SECRET", "new-secret")
		t.Setenv("OIDC_DISPLAY_NAME", "New SSO")

		seedOIDCFromEnv(s)

		// Should still have the original config
		cfg, err := s.GetOIDCConfig(ctx)
		require.NoError(t, err)
		assert.Equal(t, "https://existing.example.com", cfg.IssuerURL)
		assert.Equal(t, "existing-client", cfg.ClientID)
	})

	t.Run("skips when OIDC_CLIENT_ID missing", func(t *testing.T) {
		s := openTestStore(t)
		t.Setenv("OIDC_ISSUER", "https://accounts.example.com")
		t.Setenv("OIDC_CLIENT_ID", "")

		seedOIDCFromEnv(s)

		cfg, err := s.GetOIDCConfig(context.Background())
		require.NoError(t, err)
		assert.Empty(t, cfg.IssuerURL)
	})
}

func TestDisableAuth(t *testing.T) {
	t.Run("disables auth on existing DB", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		// Create and set up a store with auth enabled
		s := &store.Store{}
		require.NoError(t, s.Open(dbPath))
		err := s.UpdateAuthSettings(context.Background(), &model.AuthSettings{
			Enabled:  true,
			Username: "admin",
			Password: "secret123",
		})
		require.NoError(t, err)
		s.Close()

		// Use disableAuth via env var
		t.Setenv("DB_PATH", dbPath)
		require.NoError(t, disableAuth())

		// Verify auth is disabled
		s2 := &store.Store{}
		require.NoError(t, s2.Open(dbPath))
		t.Cleanup(func() { s2.Close() })

		enabled, user, passHash, err := s2.GetRawAuthRow()
		require.NoError(t, err)
		assert.Equal(t, 0, enabled)
		assert.Empty(t, user)
		assert.Empty(t, passHash)
	})

	t.Run("returns error for invalid DB path", func(t *testing.T) {
		t.Setenv("DB_PATH", "/nonexistent/path/to/db.sqlite")
		err := disableAuth()
		assert.Error(t, err)
	})
}
