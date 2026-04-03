package main

import (
	"html/template"
	"log/slog"
	"net/http"
)

func handleGrid(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := store.GetGridData()
		if err != nil {
			slog.Error("failed to load grid data", "err", err)
			data = &GridData{Error: "Nie udalo sie zaladowac polki -- sprawdz logi serwera."}
		}

		tmpl, err := template.ParseFS(mustSubFS(contentFS, "templates"),
			"layout.html", "grid.html")
		if err != nil {
			slog.Error("template parse error", "err", err)
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
			slog.Error("template execute error", "err", err)
		}
	}
}
