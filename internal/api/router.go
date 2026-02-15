package api

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rflorenc/ansible-automation-workbench/internal/models"
)

// Server holds shared state for all API handlers.
type Server struct {
	Connections *models.ConnectionStore
	Jobs        *models.JobStore
	Previews    *PreviewStore
}

// NewRouter builds the chi router with all API routes and static file serving.
func NewRouter(s *Server, webFS fs.FS) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Connections
		r.Post("/connections", s.CreateConnection)
		r.Get("/connections", s.ListConnections)
		r.Put("/connections/{id}", s.UpdateConnection)
		r.Delete("/connections/{id}", s.DeleteConnection)
		r.Post("/connections/{id}/test", s.TestConnection)

		// Resource browsing
		r.Get("/connections/{id}/resources", s.ListResourceTypes)
		r.Get("/connections/{id}/resources/{type}", s.ListResourcesOfType)

		// Operations (async)
		r.Post("/connections/{id}/cleanup", s.RunCleanup)
		r.Post("/connections/{id}/populate", s.RunPopulate)
		r.Post("/connections/{id}/export", s.RunExport)

		// Migration
		r.Post("/migrate/preview", s.MigrationPreviewHandler)
		r.Get("/migrate/preview/{jobId}", s.GetMigrationPreview)
		r.Post("/migrate/run", s.MigrationRunHandler)

		// Jobs
		r.Get("/jobs", s.ListJobs)
		r.Get("/jobs/{id}", s.GetJob)
	})

	// WebSocket (outside /api to avoid JSON content-type assumptions)
	r.Get("/ws/jobs/{id}/logs", s.StreamJobLogs)

	// Serve embedded frontend (catch-all)
	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Try to serve the actual file (JS, CSS, fonts, etc.)
		f, err := webFS.Open(path[1:])
		if err == nil {
			f.Close()
			http.ServeFileFS(w, req, webFS, path[1:])
			return
		}

		// For any non-file path, serve index.html (SPA client-side routing)
		http.ServeFileFS(w, req, webFS, "index.html")
	})

	return r
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
