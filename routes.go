package main

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func newRouter(store *Store) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Get("/", handleGrid(store))

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Route("/items", func(r chi.Router) {
			r.Get("/", handleListItems(store))
			r.Post("/", handleCreateItem(store))
			r.Route("/{itemID}", func(r chi.Router) {
				r.Get("/", handleGetItem(store))
				r.Put("/", handleUpdateItem(store))
				r.Delete("/", handleDeleteItem(store))
				r.Post("/tags", handleAddTag(store))
				r.Delete("/tags/{tagName}", handleRemoveTag(store))
			})
		})
		r.Get("/tags", handleListTags(store))
		r.Get("/search", handleSearch(store))
		r.Get("/containers/{containerID}/items", handleListContainerItems(store))
		r.Put("/shelf/resize", handleResizeShelf(store))
	})

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
