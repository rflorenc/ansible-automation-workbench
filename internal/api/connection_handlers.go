package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rflorenc/ansible-automation-workbench/internal/models"
	"github.com/rflorenc/ansible-automation-workbench/internal/platform"
)

func (s *Server) CreateConnection(w http.ResponseWriter, r *http.Request) {
	var conn models.Connection
	if err := json.NewDecoder(r.Body).Decode(&conn); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if conn.Host == "" {
		writeError(w, http.StatusBadRequest, "host is required")
		return
	}
	if conn.Type == "" {
		conn.Type = "awx"
	}
	if conn.Role == "" {
		if conn.Type == "awx" {
			conn.Role = "source"
		} else {
			conn.Role = "destination"
		}
	}
	if conn.Scheme == "" {
		conn.Scheme = "https"
	}
	if conn.Port == 0 {
		if conn.Scheme == "https" {
			conn.Port = 443
		} else {
			conn.Port = 80
		}
	}
	s.Connections.Create(&conn)
	writeJSON(w, http.StatusCreated, conn)
}

func (s *Server) ListConnections(w http.ResponseWriter, r *http.Request) {
	conns := s.Connections.List()
	writeJSON(w, http.StatusOK, conns)
}

func (s *Server) UpdateConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var conn models.Connection
	if err := json.NewDecoder(r.Body).Decode(&conn); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	conn.ID = id
	if !s.Connections.Update(&conn) {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}
	writeJSON(w, http.StatusOK, conn)
}

func (s *Server) DeleteConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !s.Connections.Delete(id) {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) TestConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	conn := s.Connections.Get(id)
	if conn == nil {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}
	p := platform.NewPlatform(conn)
	err := p.Ping()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}
