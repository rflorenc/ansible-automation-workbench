package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/rflorenc/ansible-automation-workbench/internal/platform"
)

func (s *Server) RunCleanup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	conn := s.Connections.Get(id)
	if conn == nil {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}

	jobType := conn.Type + "-cleanup"
	job := s.Jobs.Create(jobType, id)
	p := platform.NewPlatform(conn)

	go func() {
		job.AppendLog(fmt.Sprintf("Cleaning up %s (%s)", conn.Name, conn.BaseURL()))
		err := p.Cleanup(job.AppendLog)
		if err != nil {
			job.AppendLog("ERROR: " + err.Error())
			job.Fail(err.Error())
		} else {
			job.Complete()
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": job.ID})
}

func (s *Server) RunPopulate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	conn := s.Connections.Get(id)
	if conn == nil {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}

	jobType := conn.Type + "-populate"
	job := s.Jobs.Create(jobType, id)
	p := platform.NewPlatform(conn)

	go func() {
		job.AppendLog(fmt.Sprintf("Populating %s (%s)", conn.Name, conn.BaseURL()))
		err := p.Populate(job.AppendLog)
		if err != nil {
			job.AppendLog("ERROR: " + err.Error())
			job.Fail(err.Error())
		} else {
			job.Complete()
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": job.ID})
}

func (s *Server) RunExport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	conn := s.Connections.Get(id)
	if conn == nil {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}

	// Create export output dir
	outputDir := filepath.Join(os.TempDir(), "migration-tool-export", id)
	os.MkdirAll(outputDir, 0755)

	jobType := conn.Type + "-export"
	job := s.Jobs.Create(jobType, id)
	p := platform.NewPlatform(conn)

	go func() {
		job.AppendLog(fmt.Sprintf("Exporting %s (%s)", conn.Name, conn.BaseURL()))
		job.AppendLog("Exporting to: " + outputDir)
		err := p.Export(outputDir, job.AppendLog)
		if err != nil {
			job.AppendLog("ERROR: " + err.Error())
			job.Fail(err.Error())
		} else {
			job.Complete()
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"job_id":     job.ID,
		"output_dir": outputDir,
	})
}
