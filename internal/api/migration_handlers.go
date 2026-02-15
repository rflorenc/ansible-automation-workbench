package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/rflorenc/ansible-automation-workbench/internal/migration"
	"github.com/rflorenc/ansible-automation-workbench/internal/models"
)

// previewCache holds the preview result and exported data between
// the preview and run steps.
type previewCache struct {
	Preview    *models.MigrationPreview
	ExportData *migration.ExportedData
}

// PreviewStore provides thread-safe storage for migration previews.
type PreviewStore struct {
	mu       sync.RWMutex
	previews map[string]*previewCache
}

func NewPreviewStore() *PreviewStore {
	return &PreviewStore{previews: make(map[string]*previewCache)}
}

func (ps *PreviewStore) Store(jobID string, pc *previewCache) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.previews[jobID] = pc
}

func (ps *PreviewStore) Get(jobID string) *previewCache {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.previews[jobID]
}

func (ps *PreviewStore) Delete(jobID string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.previews, jobID)
}

// MigrationPreviewHandler starts an async preview job (export + preflight).
func (s *Server) MigrationPreviewHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceID      string `json:"source_id"`
		DestinationID string `json:"destination_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	src := s.Connections.Get(req.SourceID)
	if src == nil {
		writeError(w, http.StatusNotFound, "source connection not found")
		return
	}
	dst := s.Connections.Get(req.DestinationID)
	if dst == nil {
		writeError(w, http.StatusNotFound, "destination connection not found")
		return
	}

	job := s.Jobs.Create("migration-preview", req.SourceID)

	go func() {
		preview, data, err := migration.Preview(src, dst, job.AppendLog)
		if err != nil {
			job.AppendLog("ERROR: " + err.Error())
			job.Fail(err.Error())
			return
		}

		s.Previews.Store(job.ID, &previewCache{
			Preview:    preview,
			ExportData: data,
		})

		job.Complete()
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": job.ID})
}

// GetMigrationPreview returns the cached preview result for a completed preview job.
func (s *Server) GetMigrationPreview(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")

	job := s.Jobs.Get(jobID)
	if job == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	if job.Status == "running" {
		writeJSON(w, http.StatusConflict, map[string]string{
			"status":  "running",
			"message": "preview is still in progress",
		})
		return
	}

	if job.Status == "failed" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status": "failed",
			"error":  job.Error,
		})
		return
	}

	cached := s.Previews.Get(jobID)
	if cached == nil {
		writeError(w, http.StatusNotFound, "preview data not found")
		return
	}

	writeJSON(w, http.StatusOK, cached.Preview)
}

// MigrationRunHandler starts the import from a previously cached preview.
func (s *Server) MigrationRunHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceID      string `json:"source_id"`
		DestinationID string `json:"destination_id"`
		PreviewJobID  string `json:"preview_job_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	cached := s.Previews.Get(req.PreviewJobID)
	if cached == nil {
		writeError(w, http.StatusNotFound, "preview not found — run preview first")
		return
	}

	dst := s.Connections.Get(req.DestinationID)
	if dst == nil {
		writeError(w, http.StatusNotFound, "destination connection not found")
		return
	}

	job := s.Jobs.Create("migration-run", req.DestinationID)

	go func() {
		err := migration.Run(dst, cached.ExportData, cached.Preview, job.AppendLog)
		if err != nil {
			job.AppendLog("ERROR: " + err.Error())
			job.Fail(err.Error())
		} else {
			job.Complete()
		}
		// Clean up preview cache after migration completes
		s.Previews.Delete(req.PreviewJobID)
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": job.ID})
}
