package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
	srv := server.NewServer(s, mgr)
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
