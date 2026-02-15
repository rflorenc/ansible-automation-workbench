package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) ListJobs(w http.ResponseWriter, r *http.Request) {
	jobs := s.Jobs.List()
	writeJSON(w, http.StatusOK, jobs)
}

func (s *Server) GetJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	job := s.Jobs.Get(id)
	if job == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}
