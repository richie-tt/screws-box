package main

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func newRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Get("/", handleIndex)

	r.Handle("/static/*", http.StripPrefix("/static/",
		http.FileServerFS(mustSubFS(contentFS, "static"))))

	return r
}

func mustSubFS(parent fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(parent, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
