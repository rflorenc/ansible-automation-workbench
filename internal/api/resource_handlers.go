package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rflorenc/ansible-automation-workbench/internal/models"
	"github.com/rflorenc/ansible-automation-workbench/internal/platform"
)

func (s *Server) ListResourceTypes(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	conn := s.Connections.Get(id)
	if conn == nil {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}
	p := platform.NewPlatform(conn)
	writeJSON(w, http.StatusOK, p.GetResourceTypes())
}

func (s *Server) ListResourcesOfType(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	resourceType := chi.URLParam(r, "type")
	conn := s.Connections.Get(id)
	if conn == nil {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}
	p := platform.NewPlatform(conn)
	resources, err := p.ListResources(resourceType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Ensure we return [] not null for empty results
	if resources == nil {
		resources = []models.Resource{}
	}
	writeJSON(w, http.StatusOK, resources)
}
