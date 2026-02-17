package platform

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rflorenc/ansible-automation-workbench/internal/models"
)

func newTestClient(ts *httptest.Server) *Client {
	return &Client{
		baseURL:    ts.URL,
		username:   "admin",
		password:   "secret",
		httpClient: ts.Client(),
	}
}

func TestClient_Get_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	body, err := c.Get("/api/v2/ping/", nil)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if string(body) != `{"status":"ok"}` {
		t.Errorf("body = %q, want {\"status\":\"ok\"}", string(body))
	}
}

func TestClient_Get_AuthHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			t.Errorf("BasicAuth = (%q, %q, %v), want (admin, secret, true)", user, pass, ok)
		}
		w.Write([]byte("{}"))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Get("/test", nil)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
}

func TestClient_Get_ErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"detail":"Invalid username/password."}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Get("/api/v2/me/", nil)
	if err == nil {
		t.Fatal("Get should return error for 401")
	}
}

func TestClient_GetAll_Pagination(t *testing.T) {
	page := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		var resp map[string]interface{}
		if page == 1 {
			resp = map[string]interface{}{
				"count":   3,
				"next":    "/api/v2/orgs/?page=2",
				"results": []interface{}{map[string]interface{}{"id": 1, "name": "Org1"}},
			}
		} else {
			resp = map[string]interface{}{
				"count":   3,
				"next":    nil,
				"results": []interface{}{map[string]interface{}{"id": 2, "name": "Org2"}, map[string]interface{}{"id": 3, "name": "Org3"}},
			}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	results, err := c.GetAll("/api/v2/orgs/")
	if err != nil {
		t.Fatalf("GetAll returned error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("GetAll returned %d results, want 3", len(results))
	}
	if results[0]["name"] != "Org1" {
		t.Errorf("results[0].name = %v, want Org1", results[0]["name"])
	}
}

func TestClient_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":1}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	body, status, err := c.Post("/api/v2/organizations/", map[string]string{"name": "Test"})
	if err != nil {
		t.Fatalf("Post returned error: %v", err)
	}
	if status != 201 {
		t.Errorf("status = %d, want 201", status)
	}
	if string(body) != `{"id":1}` {
		t.Errorf("body = %q", string(body))
	}
}

func TestClient_Delete(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	err := c.Delete("/api/v2/organizations/1/")
	if err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}

func TestClient_Delete_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	err := c.Delete("/api/v2/organizations/999/")
	if err != nil {
		t.Fatalf("Delete(404) should not error, got: %v", err)
	}
}

func TestClient_Ping(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ha":true}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	err := c.Ping("/api/v2/ping/")
	if err != nil {
		t.Fatalf("Ping returned error: %v", err)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		expect string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"long", "hello world", 5, "hello..."},
		{"empty", "", 5, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncate(tc.input, tc.maxLen)
			if got != tc.expect {
				t.Errorf("truncate(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.expect)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	conn := &models.Connection{
		Scheme:   "https",
		Host:     "example.com",
		Port:     443,
		Username: "user",
		Password: "pass",
		Insecure: true,
	}
	c := NewClient(conn)
	if c.baseURL != "https://example.com:443" {
		t.Errorf("baseURL = %q, want https://example.com:443", c.baseURL)
	}
	if c.username != "user" || c.password != "pass" {
		t.Error("credentials not set correctly")
	}
}
