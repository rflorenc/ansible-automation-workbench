package models

import (
	"fmt"
	"sync"

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
	Insecure bool   `json:"insecure"` // skip TLS verification
}

// BaseURL returns the full base URL for this connection.
func (c *Connection) BaseURL() string {
	return fmt.Sprintf("%s://%s:%d", c.Scheme, c.Host, c.Port)
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
	s.conns[c.ID] = c
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
