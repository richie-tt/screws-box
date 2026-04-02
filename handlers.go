package main

import (
	"html/template"
	"log/slog"
	"net/http"
)

func handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(mustSubFS(contentFS, "templates"),
		"layout.html", "index.html")
	if err != nil {
		slog.Error("template parse error", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, nil); err != nil {
		slog.Error("template execute error", "err", err)
	}
}
