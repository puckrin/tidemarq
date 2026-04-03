package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/tidemarq/tidemarq/internal/auth"
)

// Routes builds and returns the application router.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	// Public endpoints.
	r.Get("/health", s.handleHealth)
	r.Post("/api/v1/auth/login", s.handleLogin)

	// Authenticated endpoints.
	r.Group(func(r chi.Router) {
		r.Use(s.authSvc.Middleware)

		// Admin-only: user management.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole("admin"))

			r.Get("/api/v1/users", s.handleListUsers)
			r.Post("/api/v1/users", s.handleCreateUser)
			r.Get("/api/v1/users/{id}", s.handleGetUser)
			r.Put("/api/v1/users/{id}", s.handleUpdateUser)
			r.Delete("/api/v1/users/{id}", s.handleDeleteUser)
		})

		// Job management: read access for all authenticated users; write for admin/operator.
		r.Get("/api/v1/jobs", s.handleListJobs)
		r.Get("/api/v1/jobs/{id}", s.handleGetJob)

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole("admin", "operator"))

			r.Post("/api/v1/jobs", s.handleCreateJob)
			r.Post("/api/v1/jobs/{id}/run", s.handleRunJob)
		})
	})

	// Static frontend — serves frontend/dist in production; holding page otherwise.
	r.Handle("/*", staticHandler(s.cfg.Server.StaticDir))

	return r
}
