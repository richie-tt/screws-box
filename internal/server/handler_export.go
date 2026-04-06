package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

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
