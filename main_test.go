package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGridHandler(t *testing.T) {
	store := openTestStore(t)
	router := newRouter(store)
	ts := httptest.NewServer(router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET / status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	html := string(bodyBytes)

	if !strings.Contains(html, "Screws Box") {
		t.Error("response body missing 'Screws Box'")
	}
	if !strings.Contains(html, "grid-container") {
		t.Error("response body missing 'grid-container'")
	}
	if !strings.Contains(html, "1A") {
		t.Error("response body missing '1A' (first cell coord)")
	}
	if !strings.Contains(html, "10E") {
		t.Error("response body missing '10E' (last cell coord for 10x5 grid)")
	}
	if !strings.Contains(html, "cell-coord") {
		t.Error("response body missing 'cell-coord'")
	}
	if !strings.Contains(html, "cell-count") {
		t.Error("response body missing 'cell-count'")
	}
	if strings.Contains(html, "Setup in progress") {
		t.Error("response body should NOT contain 'Setup in progress' (old placeholder)")
	}
}

func TestStaticCSS(t *testing.T) {
	store := openTestStore(t)
	router := newRouter(store)
	ts := httptest.NewServer(router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/static/css/app.css")
	if err != nil {
		t.Fatalf("GET static CSS failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET static CSS status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestServerBindAddress(t *testing.T) {
	store := openTestStore(t)

	// Find a free port
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	srv := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: newRouter(store),
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Errorf("server error: %v", err)
		}
	}()
	defer srv.Close()

	// Wait briefly for server to start
	time.Sleep(50 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	if err != nil {
		t.Fatalf("could not reach server on port %d: %v", port, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestGracefulShutdown(t *testing.T) {
	store := openTestStore(t)

	srv := &http.Server{
		Addr:    "0.0.0.0:0",
		Handler: newRouter(store),
	}

	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(listener)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Initiate graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	// Server should return ErrServerClosed
	serveErr := <-errCh
	if serveErr != http.ErrServerClosed {
		t.Errorf("Serve() = %v, want ErrServerClosed", serveErr)
	}
}
