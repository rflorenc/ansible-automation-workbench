package models

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Connection represents a user-configured AWX or AAP instance.
type Connection struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`     // "awx" or "aap"
	Role     string `json:"role"`     // "source" or "destination"
	Scheme   string `json:"scheme"`   // "http" or "https"
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Insecure    bool       `json:"insecure"`                // skip TLS verification
	Version     string     `json:"version,omitempty"`       // detected platform version, e.g. "23.4.0" or "4.7.8"
	APIPrefix   string     `json:"api_prefix,omitempty"`    // detected API prefix, e.g. "/api/v2/" or "/api/controller/v2/"
	PingStatus  string     `json:"ping_status"`             // "unknown", "ok", "error"
	PingError   string     `json:"ping_error,omitempty"`
	AuthStatus  string     `json:"auth_status"`             // "unknown", "ok", "error"
	AuthError   string     `json:"auth_error,omitempty"`
	LastChecked *time.Time `json:"last_checked,omitempty"`
}

// BaseURL returns the full base URL for this connection.
func (c *Connection) BaseURL() string {
	return fmt.Sprintf("%s://%s:%d", c.Scheme, c.Host, c.Port)
}

// MaskedPassword returns a mask if password is set, empty string otherwise.
func (c *Connection) MaskedPassword() string {
	if c.Password != "" {
		return "••••••••"
	}
	return ""
}

// ConnectionStore is an in-memory thread-safe store for connections.
type ConnectionStore struct {
	mu    sync.RWMutex
	conns map[string]*Connection
}

// NewConnectionStore creates an empty connection store.
func NewConnectionStore() *ConnectionStore {
	return &ConnectionStore{conns: make(map[string]*Connection)}
}

// Create adds a new connection, assigning it a UUID.
func (s *ConnectionStore) Create(c *Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.ID = uuid.New().String()
	c.PingStatus = "unknown"
	c.AuthStatus = "unknown"
	s.conns[c.ID] = c
}

// SetHealth updates the ping and auth status of a connection.
func (s *ConnectionStore) SetHealth(id, pingStatus, pingError, authStatus, authError string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	conn, ok := s.conns[id]
	if !ok {
		return
	}
	now := time.Now()
	conn.PingStatus = pingStatus
	conn.PingError = pingError
	conn.AuthStatus = authStatus
	conn.AuthError = authError
	conn.LastChecked = &now
}

// SetVersion updates the detected version and API prefix of a connection.
func (s *ConnectionStore) SetVersion(id, version, apiPrefix string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	conn, ok := s.conns[id]
	if !ok {
		return
	}
	conn.Version = version
	conn.APIPrefix = apiPrefix
}

// Get returns a connection by ID, or nil if not found.
func (s *ConnectionStore) Get(id string) *Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.conns[id]
}

// List returns all connections.
func (s *ConnectionStore) List() []*Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Connection, 0, len(s.conns))
	for _, c := range s.conns {
		result = append(result, c)
	}
	return result
}

// Update replaces an existing connection's settings.
func (s *ConnectionStore) Update(c *Connection) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.conns[c.ID]; !ok {
		return false
	}
	s.conns[c.ID] = c
	return true
}

// Delete removes a connection by ID.
func (s *ConnectionStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.conns[id]; !ok {
		return false
	}
	delete(s.conns, id)
	return true
}
