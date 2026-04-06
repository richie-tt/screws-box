package server

import (
	"io/fs"
	"net/http"

	"screws-box/internal/session"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server holds application dependencies for HTTP handlers.
type Server struct {
	store    StoreService
	sessions *session.Manager
}

// NewServer creates a Server with the given dependencies.
func NewServer(store StoreService, sessions *session.Manager) *Server {
	return &Server{store: store, sessions: sessions}
}

// Router creates the chi router with all routes.
func (srv *Server) Router() http.Handler {
	r := chi.NewRouter()

	// Healthcheck — outside logging middleware so K8s probes don't flood logs.
	r.Get("/healthz", srv.handleHealthz())

	// All application routes with full middleware stack.
	r.Group(func(r chi.Router) {
		r.Use(middleware.RealIP)
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)
		r.Use(middleware.RequestID)
		r.Use(newRateLimitAPI())

		// Public routes (no auth required)
		r.Get("/login", srv.handleLoginPage())
		r.With(newRateLimitLogin()).Post("/login", srv.handleLoginPost())
		r.Get("/logout", srv.handleLogout())

		// OIDC routes (public -- callback must not be behind authMiddleware)
		r.Get("/auth/oidc", srv.handleOIDCStart())
		r.With(newRateLimitLogin()).Get("/auth/callback", srv.handleOIDCCallback())

		r.Handle("/static/*", http.StripPrefix("/static/",
			http.FileServerFS(mustSubFS(ContentFS, "static"))))

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(srv.authMiddleware())
			r.Use(srv.csrfProtect())

			r.Get("/", srv.handleGrid())
			r.Get("/settings", srv.handleSettings())

			r.Route("/api", func(r chi.Router) {
				r.Route("/items", func(r chi.Router) {
					r.Get("/", srv.handleListItems())
					r.Post("/", srv.handleCreateItem())
					r.Route("/{itemID}", func(r chi.Router) {
						r.Get("/", srv.handleGetItem())
						r.Put("/", srv.handleUpdateItem())
						r.Delete("/", srv.handleDeleteItem())
						r.Post("/tags", srv.handleAddTag())
						r.Delete("/tags/{tagName}", srv.handleRemoveTag())
					})
				})
				r.Get("/tags", srv.handleListTags())
				r.Route("/tags/{tagID}", func(r chi.Router) {
					r.Put("/", srv.handleRenameTag())
					r.Delete("/", srv.handleDeleteTag())
				})
				r.Get("/search", srv.handleSearch())
				r.Get("/containers/{containerID}/items", srv.handleListContainerItems())
				r.Put("/shelf/resize", srv.handleResizeShelf())
				r.Get("/shelf/auth", srv.handleGetAuthSettings())
				r.Put("/shelf/auth", srv.handleUpdateAuthSettings())
				r.Get("/oidc/config", srv.handleGetOIDCConfig())
				r.Put("/oidc/config", srv.handleUpdateOIDCConfig())
				r.Get("/export", srv.handleExport())
				r.Post("/import/validate", srv.handleImportValidate())
				r.Post("/import/confirm", srv.handleImportConfirm())
				r.Get("/sessions", srv.handleListSessions())
				r.Delete("/sessions", srv.handleRevokeAllOthers())
				r.Delete("/sessions/{sessionID}", srv.handleRevokeSession())
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
