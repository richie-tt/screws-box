package main

import (
	"net"
	"os"
	"path/filepath"
	"screws-box/internal/store"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunOpenStoreFails(t *testing.T) {
	// A path under a non-existent directory cannot be created.
	t.Setenv("DB_PATH", "/nonexistent/parent/dir/run_test.db")
	t.Setenv("REDIS_URL", "")
	t.Setenv("OIDC_ISSUER", "")

	err := run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open store")
}

func TestRunRedisStoreFails(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("DB_PATH", filepath.Join(tmpDir, "run_test.db"))
	// ParseURL on bare string fails immediately, no network round-trip.
	t.Setenv("REDIS_URL", "not-a-redis-url")
	t.Setenv("SESSION_TTL", "1h")
	t.Setenv("OIDC_ISSUER", "")

	err := run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Redis")
}

func TestRunServerListenError(t *testing.T) {
	// Occupy a port for the duration of run() so ListenAndServe fails.
	listener, err := net.Listen("tcp", "0.0.0.0:0") //nolint:gosec // G102: test occupies port to force bind failure
	require.NoError(t, err)
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	tmpDir := t.TempDir()
	t.Setenv("DB_PATH", filepath.Join(tmpDir, "run_test.db"))
	t.Setenv("PORT", strconv.Itoa(port))
	t.Setenv("REDIS_URL", "")
	t.Setenv("SESSION_TTL", "1h")
	t.Setenv("OIDC_ISSUER", "")

	err = run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server error")
}

func TestRunGracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("DB_PATH", filepath.Join(tmpDir, "run_test.db"))
	t.Setenv("PORT", "0") // bind any free port
	t.Setenv("REDIS_URL", "")
	t.Setenv("SESSION_TTL", "1h")
	t.Setenv("OIDC_ISSUER", "")

	errCh := make(chan error, 1)
	go func() {
		errCh <- run()
	}()

	// Give run() time to install its SIGTERM handler before signalling.
	// 200ms is generous; CI machines under load can be slow.
	time.Sleep(200 * time.Millisecond)

	// signal.NotifyContext suppresses the default SIGTERM handler while
	// run() is registered, so this only cancels run()'s context.
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("run() did not return within 10s after SIGTERM")
	}
}

func TestSeedOIDCFromEnvSaveError(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "seed_err.db")
	s := &store.Store{}
	require.NoError(t, s.Open(dbPath))
	require.NoError(t, s.Close())

	t.Setenv("OIDC_ISSUER", "https://example.com")
	t.Setenv("OIDC_CLIENT_ID", "cid")
	t.Setenv("OIDC_CLIENT_SECRET", "secret")
	t.Setenv("OIDC_DISPLAY_NAME", "Example")

	// Closed store: GetOIDCConfig errs (ignored), SaveOIDCConfig errs
	// (logged). Function must return cleanly without panic.
	require.NotPanics(t, func() { seedOIDCFromEnv(s) })
}

func TestMainDisableAuthFlag(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "main_test.db")

	// Pre-create DB so disableAuth() succeeds and main() returns
	// without calling os.Exit (which would terminate the test runner).
	s := &store.Store{}
	require.NoError(t, s.Open(dbPath))
	require.NoError(t, s.Close())

	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"screwsbox", "--disable-auth"}
	t.Setenv("DB_PATH", dbPath)

	require.NotPanics(t, main)
}
