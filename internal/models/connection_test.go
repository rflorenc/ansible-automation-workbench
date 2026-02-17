package models

import (
	"sync"
	"testing"
)

func TestBaseURL(t *testing.T) {
	tests := []struct {
		name   string
		conn   Connection
		expect string
	}{
		{"https default", Connection{Scheme: "https", Host: "aap.lab.local", Port: 443}, "https://aap.lab.local:443"},
		{"http custom port", Connection{Scheme: "http", Host: "awx.lab.local", Port: 32000}, "http://awx.lab.local:32000"},
		{"localhost", Connection{Scheme: "http", Host: "localhost", Port: 80}, "http://localhost:80"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.conn.BaseURL()
			if got != tc.expect {
				t.Errorf("BaseURL() = %q, want %q", got, tc.expect)
			}
		})
	}
}

func TestMaskedPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		expect   string
	}{
		{"non-empty", "secret123", "••••••••"},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Connection{Password: tc.password}
			got := c.MaskedPassword()
			if got != tc.expect {
				t.Errorf("MaskedPassword() = %q, want %q", got, tc.expect)
			}
		})
	}
}

func TestConnectionStore_CRUD(t *testing.T) {
	store := NewConnectionStore()

	// Create
	conn := &Connection{Name: "test-awx", Type: "awx", Host: "localhost"}
	store.Create(conn)
	if conn.ID == "" {
		t.Fatal("Create did not assign an ID")
	}
	if conn.PingStatus != "unknown" {
		t.Errorf("Create should set PingStatus to 'unknown', got %q", conn.PingStatus)
	}
	if conn.AuthStatus != "unknown" {
		t.Errorf("Create should set AuthStatus to 'unknown', got %q", conn.AuthStatus)
	}

	// Get
	got := store.Get(conn.ID)
	if got == nil || got.Name != "test-awx" {
		t.Fatalf("Get(%s) returned %v", conn.ID, got)
	}

	// Get not found
	if store.Get("nonexistent") != nil {
		t.Error("Get(nonexistent) should return nil")
	}

	// List
	list := store.List()
	if len(list) != 1 {
		t.Fatalf("List() returned %d items, want 1", len(list))
	}

	// Update
	conn.Name = "updated"
	if !store.Update(conn) {
		t.Fatal("Update returned false for existing connection")
	}
	if store.Get(conn.ID).Name != "updated" {
		t.Error("Update did not persist name change")
	}

	// Update not found
	missing := &Connection{ID: "missing"}
	if store.Update(missing) {
		t.Error("Update should return false for missing ID")
	}

	// Delete
	if !store.Delete(conn.ID) {
		t.Fatal("Delete returned false for existing connection")
	}
	if store.Get(conn.ID) != nil {
		t.Error("Get after Delete should return nil")
	}

	// Delete not found
	if store.Delete("missing") {
		t.Error("Delete should return false for missing ID")
	}
}

func TestConnectionStore_SetHealth(t *testing.T) {
	store := NewConnectionStore()
	conn := &Connection{Name: "test", Host: "localhost"}
	store.Create(conn)

	store.SetHealth(conn.ID, "ok", "", "ok", "")
	got := store.Get(conn.ID)
	if got.PingStatus != "ok" {
		t.Errorf("PingStatus = %q, want %q", got.PingStatus, "ok")
	}
	if got.AuthStatus != "ok" {
		t.Errorf("AuthStatus = %q, want %q", got.AuthStatus, "ok")
	}
	if got.LastChecked == nil {
		t.Error("LastChecked should be set after SetHealth")
	}

	store.SetHealth(conn.ID, "ok", "", "error", "bad credentials")
	got = store.Get(conn.ID)
	if got.PingStatus != "ok" {
		t.Errorf("PingStatus = %q, want %q", got.PingStatus, "ok")
	}
	if got.AuthStatus != "error" || got.AuthError != "bad credentials" {
		t.Errorf("SetHealth(auth error) = (%q, %q), want (error, bad credentials)", got.AuthStatus, got.AuthError)
	}

	store.SetHealth(conn.ID, "error", "connection refused", "unknown", "")
	got = store.Get(conn.ID)
	if got.PingStatus != "error" || got.PingError != "connection refused" {
		t.Errorf("SetHealth(ping error) = (%q, %q), want (error, connection refused)", got.PingStatus, got.PingError)
	}

	// SetHealth on missing ID should not panic
	store.SetHealth("nonexistent", "ok", "", "ok", "")
}

func TestConnectionStore_Concurrent(t *testing.T) {
	store := NewConnectionStore()
	var wg sync.WaitGroup

	// Concurrent creates
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := &Connection{Name: "concurrent", Host: "localhost"}
			store.Create(c)
		}()
	}
	wg.Wait()

	list := store.List()
	if len(list) != 50 {
		t.Fatalf("expected 50 connections, got %d", len(list))
	}

	// Concurrent reads + status updates
	for _, c := range list {
		wg.Add(2)
		go func(id string) {
			defer wg.Done()
			store.Get(id)
		}(c.ID)
		go func(id string) {
			defer wg.Done()
			store.SetHealth(id, "ok", "", "ok", "")
		}(c.ID)
	}
	wg.Wait()
}
