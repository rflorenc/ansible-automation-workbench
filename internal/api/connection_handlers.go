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
	resp := conn
	resp.Password = conn.MaskedPassword()
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) ListConnections(w http.ResponseWriter, r *http.Request) {
	conns := s.Connections.List()
	// Return copies with masked passwords
	masked := make([]models.Connection, len(conns))
	for i, c := range conns {
		masked[i] = *c
		masked[i].Password = c.MaskedPassword()
	}
	writeJSON(w, http.StatusOK, masked)
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
	resp := conn
	resp.Password = conn.MaskedPassword()
	writeJSON(w, http.StatusOK, resp)
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
	client := platform.NewClient(conn)

	// Step 1: connectivity check (unauthenticated)
	pingStatus, pingError := "ok", ""
	if err := p.Ping(); err != nil {
		pingStatus = "error"
		pingError = err.Error()
	}

	// Step 2: credential check (authenticated)
	authStatus, authError := "unknown", ""
	version := conn.Version
	if pingStatus == "ok" {
		if conn.Username == "" || conn.Password == "" {
			authStatus = "error"
			authError = "no credentials configured"
		} else if err := p.CheckAuth(); err != nil {
			authStatus = "error"
			authError = err.Error()
		} else {
			authStatus = "ok"

			// Step 3: discovery (only after auth succeeds)
			var pingResp *platform.PingResponse
			for _, pp := range platform.PingPaths(conn.Type) {
				pingResp, err = client.PingWithVersion(pp)
				if err == nil {
					break
				}
			}
			if err == nil && pingResp.Version != "" {
				version = pingResp.Version
				conn.Version = version
				s.Connections.SetVersion(id, version, "")
			}
			platform.DiscoverAndStore(client, conn, s.Connections)
		}
	}

	s.Connections.SetHealth(id, pingStatus, pingError, authStatus, authError)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ping_ok":    pingStatus == "ok",
		"ping_error": pingError,
		"auth_ok":    authStatus == "ok",
		"auth_error": authError,
		"version":    version,
	})
}
