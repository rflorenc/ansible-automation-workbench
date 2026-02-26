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

// CancelJob cancels a running job.
func (s *Server) CancelJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	job := s.Jobs.Get(id)
	if job == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if job.Status != "running" {
		writeError(w, http.StatusConflict, "job is not running")
		return
	}
	job.Cancel()
	job.AppendLog("CANCELLED: migration stopped by user")
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}
