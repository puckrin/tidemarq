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

		// Conflicts: read for all authenticated; resolve/clear for admin/operator.
		r.Get("/api/v1/conflicts", s.handleListConflicts)
		r.Get("/api/v1/conflicts/{id}", s.handleGetConflict)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole("admin", "operator"))
			r.Post("/api/v1/conflicts/{id}/resolve", s.handleResolveConflict)
			r.Post("/api/v1/conflicts/clear-resolved", s.handleClearResolvedConflicts)
		})

		// Versions and quarantine: read for all; restore/clear for admin/operator.
		r.Get("/api/v1/versions", s.handleListVersions)
		r.Get("/api/v1/quarantine", s.handleListQuarantine)
		r.Get("/api/v1/quarantine/removed", s.handleListRemovedQuarantine)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole("admin", "operator"))
			r.Post("/api/v1/versions/{id}/restore", s.handleRestoreVersion)
			r.Post("/api/v1/quarantine/{id}/restore", s.handleRestoreQuarantine)
			r.Delete("/api/v1/quarantine/{id}", s.handleDeleteQuarantine)
			r.Post("/api/v1/quarantine/clear-removed", s.handleClearRemovedQuarantine)
		})

		// Mounts: admin/operator write, all authenticated read.
		r.Get("/api/v1/mounts", s.handleListMounts)
		r.Get("/api/v1/mounts/{id}", s.handleGetMount)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole("admin", "operator"))
			r.Post("/api/v1/mounts", s.handleCreateMount)
			r.Put("/api/v1/mounts/{id}", s.handleUpdateMount)
			r.Delete("/api/v1/mounts/{id}", s.handleDeleteMount)
			r.Post("/api/v1/mounts/{id}/test", s.handleTestMount)
		})

		// Settings: read by any authenticated user; write is admin only.
		r.Get("/api/v1/settings", s.handleGetSettings)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole("admin"))
			r.Put("/api/v1/settings", s.handleUpdateSettings)
		})

		// Directory browser: all authenticated users can browse.
		r.Get("/api/v1/browse", s.handleBrowse)

		// Audit log: all authenticated can read; export is admin only.
		r.Get("/api/v1/audit", s.handleListAuditLog)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole("admin"))
			r.Get("/api/v1/audit/export", s.handleExportAuditLog)
		})
	})

	// Static frontend.
	r.Handle("/*", staticHandler(s.cfg.Server.StaticDir))

	return r
}
