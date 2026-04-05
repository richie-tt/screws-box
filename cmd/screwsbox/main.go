package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"screws-box/internal/server"
	"screws-box/internal/store"
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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := "0.0.0.0:" + port

	srv := &http.Server{
		Addr:              addr,
		Handler:           server.NewRouter(&s),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		slog.Info("server starting", "addr", addr) //nolint:gosec // G706: structured logging, value is a key-value pair not interpolated
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}
