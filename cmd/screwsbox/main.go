package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"screws-box/internal/server"
	"screws-box/internal/store"
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

	_, err := s.DB().Exec("UPDATE shelf SET auth_enabled = 0, auth_user = '', auth_pass = '' WHERE id = (SELECT id FROM shelf LIMIT 1)")
	if err != nil {
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
		Addr:    addr,
		Handler: server.NewRouter(&s),
	}

	go func() {
		slog.Info("server starting", "addr", addr)
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
