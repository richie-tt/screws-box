package server

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates the chi router with all routes.
func NewRouter(s StoreService) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	// Public routes (no auth required)
	r.Get("/login", handleLoginPage(s))
	r.Post("/login", handleLoginPost(s))
	r.Get("/logout", handleLogout())
	r.Get("/api/shelf/auth", handleGetAuthSettings(s))
	r.Put("/api/shelf/auth", handleUpdateAuthSettings(s))
	r.Handle("/static/*", http.StripPrefix("/static/",
		http.FileServerFS(mustSubFS(ContentFS, "static"))))

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware(s))

		r.Get("/", handleGrid(s))

		r.Route("/api", func(r chi.Router) {
			r.Route("/items", func(r chi.Router) {
				r.Get("/", handleListItems(s))
				r.Post("/", handleCreateItem(s))
				r.Route("/{itemID}", func(r chi.Router) {
					r.Get("/", handleGetItem(s))
					r.Put("/", handleUpdateItem(s))
					r.Delete("/", handleDeleteItem(s))
					r.Post("/tags", handleAddTag(s))
					r.Delete("/tags/{tagName}", handleRemoveTag(s))
				})
			})
			r.Get("/tags", handleListTags(s))
			r.Get("/search", handleSearch(s))
			r.Get("/containers/{containerID}/items", handleListContainerItems(s))
			r.Put("/shelf/resize", handleResizeShelf(s))
		})
	})

	return r
}

func mustSubFS(parent fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(parent, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
