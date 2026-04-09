package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"screws-box/internal/model"
	"screws-box/internal/server"
	"screws-box/internal/session"
	"screws-box/internal/store"
	"strings"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// CLI: --disable-auth bypasses the server and disables authentication
	for _, arg := range os.Args[1:] {
		if arg == "--disable-auth" {
			if err := disableAuth(); err != nil {
				slog.Error("disable auth failed", "err", err)
				os.Exit(1)
			}
			return
		}
	}

	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func disableAuth() error {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./screws_box.db"
	}

	var s store.Store
	if err := s.Open(dbPath); err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer s.Close()

	if err := s.DisableAuth(); err != nil {
		return fmt.Errorf("disable auth: %w", err)
	}

	fmt.Println("Authentication disabled. Username and password cleared.")
	return nil
}

func parseSessionTTL() time.Duration {
	raw := os.Getenv("SESSION_TTL")
	if raw == "" {
		return 24 * time.Hour
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		slog.Warn("invalid SESSION_TTL, using default 24h", "value", sanitizeLogValue(raw), "err", err) //nolint:gosec // env var, not user input
		return 24 * time.Hour
	}
	if d <= 0 {
		slog.Warn("SESSION_TTL must be positive, using default 24h", "value", sanitizeLogValue(raw)) //nolint:gosec // env var, not user input
		return 24 * time.Hour
	}
	return d
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./screws_box.db"
	}

	var s store.Store
	if err := s.Open(dbPath); err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer s.Close()

	seedOIDCFromEnv(&s)

	sessionTTL := parseSessionTTL()
	redisURL := os.Getenv("REDIS_URL")
	var sessionStore session.Store
	var storeType string
	if redisURL != "" {
		rs, err := session.NewRedisStore(redisURL, sessionTTL)
		if err != nil {
			return fmt.Errorf("connect to Redis: %w", err)
		}
		defer rs.Close()
		storeType = "Redis"
		sessionStore = rs
		slog.Info("session store configured", "type", "redis", "ttl", sessionTTL)
	} else {
		ms := session.NewMemoryStore(sessionTTL, sessionTTL/2)
		defer ms.Close()
		storeType = "Memory"
		sessionStore = ms
		slog.Info("session store configured", "type", "memory", "ttl", sessionTTL)
	}
	sessionMgr := session.NewManager(sessionStore, sessionTTL, storeType)

	appSrv := server.NewServer(&s, sessionMgr)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := "0.0.0.0:" + port

	srv := &http.Server{
		Addr:              addr,
		Handler:           appSrv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server starting", "addr", addr) //nolint:gosec // addr from env/default, not user input
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
	}
	slog.Info("shutting down gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}

// sanitizeLogValue strips newlines and control characters to prevent log injection.
func sanitizeLogValue(s string) string {
	r := strings.NewReplacer("\n", "", "\r", "", "\t", " ")
	return r.Replace(s)
}

func seedOIDCFromEnv(s *store.Store) {
	issuer := os.Getenv("OIDC_ISSUER")
	if issuer == "" {
		return // no env vars set
	}
	ctx := context.Background()
	// Only seed if not already configured in DB
	existing, _ := s.GetOIDCConfig(ctx)
	if existing != nil && existing.IssuerURL != "" {
		slog.Info("OIDC config already exists in DB, skipping env var seed")
		return
	}
	cfg := &model.OIDCConfig{
		Enabled:      true,
		IssuerURL:    issuer,
		ClientID:     os.Getenv("OIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		DisplayName:  os.Getenv("OIDC_DISPLAY_NAME"),
	}
	if cfg.ClientID == "" {
		slog.Warn("OIDC_ISSUER set but OIDC_CLIENT_ID missing, skipping seed")
		return
	}
	if err := s.SaveOIDCConfig(ctx, cfg); err != nil {
		slog.Error("seed OIDC config from env", "err", err)
		return
	}
	slog.Info("seeded OIDC config from environment variables", "issuer", issuer, "display_name", cfg.DisplayName) //nolint:gosec // env vars, not user input
}
