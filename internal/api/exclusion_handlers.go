package api

import (
	"net/http"

	"github.com/rflorenc/ansible-automation-workbench/internal/migration"
	"github.com/rflorenc/ansible-automation-workbench/internal/platform"
)

// GetExclusions returns the default skip lists used during migration and cleanup.
func (s *Server) GetExclusions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"migration": migration.DefaultExclusions(),
		"cleanup":   platform.CleanupExclusions(),
	})
}
