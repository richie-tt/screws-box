package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"screws-box/internal/model"
	"sync"
	"time"
)

// pendingImport holds a validated import awaiting user confirmation.
type pendingImport struct {
	data      *model.ExportData
	createdAt time.Time
}

var (
	pendingImports   = make(map[string]*pendingImport)
	pendingImportsMu sync.Mutex
)

const pendingImportTTL = 5 * time.Minute

// generateImportToken creates a cryptographically random hex token.
func generateImportToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// cleanExpiredPendingImports removes expired entries from the pending map.
// Must be called with pendingImportsMu held.
func cleanExpiredPendingImports() {
	now := time.Now()
	for token, pi := range pendingImports {
		if now.Sub(pi.createdAt) > pendingImportTTL {
			delete(pendingImports, token)
		}
	}
}

func (srv *Server) handleExport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := srv.store.ExportAllData(r.Context())
		if err != nil {
			slog.Error("export failed", "err", err)
			http.Error(w, "export failed", http.StatusInternalServerError)
			return
		}
		filename := fmt.Sprintf("screws-box-export-%s.json", time.Now().Format("2006-01-02"))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(data); err != nil {
			slog.Error("export encode failed", "err", err)
		}
	}
}

func (srv *Server) handleImportValidate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Enforce 10MB limit per D-08
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

		file, _, err := r.FormFile("file")
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
					"errors": []string{"File exceeds the 10 MB size limit."},
				})
				return
			}
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"errors": []string{"No file uploaded."},
			})
			return
		}
		defer file.Close()

		var data model.ExportData
		if err := json.NewDecoder(file).Decode(&data); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"errors": []string{"The file does not contain valid JSON."},
			})
			return
		}

		// Validate structure
		var validationErrors []string
		if data.Version != 1 {
			validationErrors = append(validationErrors, fmt.Sprintf("Unsupported file version %d. Expected version 1.", data.Version))
		}
		if data.Shelf.Name == "" {
			validationErrors = append(validationErrors, "Missing required field: shelf.name.")
		}
		if data.Shelf.Rows <= 0 {
			validationErrors = append(validationErrors, "Missing required field: shelf.rows.")
		}
		if data.Shelf.Cols <= 0 {
			validationErrors = append(validationErrors, "Missing required field: shelf.cols.")
		}

		if len(validationErrors) > 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"errors": validationErrors,
			})
			return
		}

		// Count items and tags for summary
		itemCount := 0
		tagSet := make(map[string]bool)
		for _, c := range data.Shelf.Containers {
			for _, item := range c.Items {
				itemCount++
				for _, tag := range item.Tags {
					tagSet[tag] = true
				}
			}
		}

		// Store pending import
		token, err := generateImportToken()
		if err != nil {
			slog.Error("generate import token failed", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		pendingImportsMu.Lock()
		cleanExpiredPendingImports()
		pendingImports[token] = &pendingImport{data: &data, createdAt: time.Now()}
		pendingImportsMu.Unlock()

		writeJSON(w, http.StatusOK, map[string]any{
			"token": token,
			"summary": map[string]any{
				"shelf_name": data.Shelf.Name,
				"rows":       data.Shelf.Rows,
				"cols":       data.Shelf.Cols,
				"containers": len(data.Shelf.Containers),
				"items":      itemCount,
				"tags":       len(tagSet),
			},
		})
	}
}

func (srv *Server) handleImportConfirm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "Missing import token.",
			})
			return
		}

		pendingImportsMu.Lock()
		pi, ok := pendingImports[req.Token]
		if ok {
			delete(pendingImports, req.Token)
		}
		pendingImportsMu.Unlock()

		if !ok || time.Since(pi.createdAt) > pendingImportTTL {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "Import session expired. Please validate the file again.",
			})
			return
		}

		if err := srv.store.ImportAllData(r.Context(), pi.data); err != nil {
			slog.Error("import failed", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Import failed. Existing data was not modified.",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"message": "Import complete. Data restored successfully.",
		})
	}
}
