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

	// Healthcheck — outside logging middleware so K8s probes don't flood logs.
	r.Get("/healthz", handleHealthz(s))

	// All application routes with full middleware stack.
	r.Group(func(r chi.Router) {
		r.Use(middleware.RealIP)
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)
		r.Use(middleware.RequestID)
		r.Use(newRateLimitAPI())

		// Public routes (no auth required)
		r.Get("/login", handleLoginPage(s))
		r.With(newRateLimitLogin()).Post("/login", handleLoginPost(s))
		r.Get("/logout", handleLogout())
		r.Handle("/static/*", http.StripPrefix("/static/",
			http.FileServerFS(mustSubFS(ContentFS, "static"))))

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware(s))
			r.Use(csrfProtect(s))

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
				r.Get("/shelf/auth", handleGetAuthSettings(s))
				r.Put("/shelf/auth", handleUpdateAuthSettings(s))
			})
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
