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

	// WebSocket — authenticated via short-lived token in query param.
	r.Get("/ws", s.handleWS)

	// Authenticated endpoints.
	r.Group(func(r chi.Router) {
		r.Use(s.authSvc.Middleware)

		// WS token issuance — any authenticated user.
		r.Get("/api/v1/auth/ws-token", s.handleWSToken)

		// Admin-only: user management.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole("admin"))

			r.Get("/api/v1/users", s.handleListUsers)
			r.Post("/api/v1/users", s.handleCreateUser)
			r.Get("/api/v1/users/{id}", s.handleGetUser)
			r.Put("/api/v1/users/{id}", s.handleUpdateUser)
			r.Delete("/api/v1/users/{id}", s.handleDeleteUser)
		})

		// Job management: read access for all authenticated users.
		r.Get("/api/v1/jobs", s.handleListJobs)
		r.Get("/api/v1/jobs/{id}", s.handleGetJob)

		// Job write access: admin and operator.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole("admin", "operator"))

			r.Post("/api/v1/jobs", s.handleCreateJob)
			r.Put("/api/v1/jobs/{id}", s.handleUpdateJob)
			r.Delete("/api/v1/jobs/{id}", s.handleDeleteJob)
			r.Post("/api/v1/jobs/{id}/run", s.handleRunJob)
			r.Post("/api/v1/jobs/{id}/pause", s.handlePauseJob)
			r.Post("/api/v1/jobs/{id}/resume", s.handleResumeJob)
		})
	})

	// Static frontend.
	r.Handle("/*", staticHandler(s.cfg.Server.StaticDir))

	return r
}
