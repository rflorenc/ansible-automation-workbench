package platform

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rflorenc/ansible-automation-workbench/internal/models"
)

func TestParsePingResponse_AWX(t *testing.T) {
	body := []byte(`{"version":"23.4.0","ha":false,"active_node":"awx-1"}`)
	resp, err := ParsePingResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Version != "23.4.0" {
		t.Errorf("Version = %q, want %q", resp.Version, "23.4.0")
	}
}

func TestParsePingResponse_AAP(t *testing.T) {
	body := []byte(`{"version":"4.7.8","ha":false,"active_node":"controller-1"}`)
	resp, err := ParsePingResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Version != "4.7.8" {
		t.Errorf("Version = %q, want %q", resp.Version, "4.7.8")
	}
}

func TestParsePingResponse_Empty(t *testing.T) {
	body := []byte(`{"ha":false}`)
	_, err := ParsePingResponse(body)
	if err == nil {
		t.Fatal("expected error for missing version, got nil")
	}
}

func TestParsePingResponse_InvalidJSON(t *testing.T) {
	body := []byte(`not json`)
	_, err := ParsePingResponse(body)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestParseAPIRoot_AWX(t *testing.T) {
	body := []byte(`{"description":"AWX REST API","current_version":"/api/v2/","available_versions":{"v2":"/api/v2/"}}`)
	resp, err := ParseAPIRoot(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CurrentVersion != "/api/v2/" {
		t.Errorf("CurrentVersion = %q, want %q", resp.CurrentVersion, "/api/v2/")
	}
}

func TestParseAPIRoot_AAP(t *testing.T) {
	body := []byte(`{"apis":{"controller":{"prefix":"/api/controller/"},"gateway":{"prefix":"/api/gateway/"}}}`)
	resp, err := ParseAPIRoot(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.APIs["controller"]; !ok {
		t.Fatal("expected 'controller' in APIs map")
	}
	if resp.APIs["controller"].Prefix != "/api/controller/" {
		t.Errorf("APIs[controller].Prefix = %q, want %q", resp.APIs["controller"].Prefix, "/api/controller/")
	}
}

func TestDetectAPIPrefix_AWX(t *testing.T) {
	root := &APIRootResponse{CurrentVersion: "/api/v2/"}
	got := DetectAPIPrefix(root)
	if got != "/api/v2/" {
		t.Errorf("DetectAPIPrefix = %q, want %q", got, "/api/v2/")
	}
}

func TestDetectAPIPrefix_AWX_NoTrailingSlash(t *testing.T) {
	root := &APIRootResponse{CurrentVersion: "/api/v2"}
	got := DetectAPIPrefix(root)
	if got != "/api/v2/" {
		t.Errorf("DetectAPIPrefix = %q, want %q", got, "/api/v2/")
	}
}

func TestDetectAPIPrefix_AAP(t *testing.T) {
	root := &APIRootResponse{
		APIs: map[string]APIRootServiceEntry{
			"controller": {Prefix: "/api/controller/"},
		},
	}
	got := DetectAPIPrefix(root)
	if got != "/api/controller/v2/" {
		t.Errorf("DetectAPIPrefix = %q, want %q", got, "/api/controller/v2/")
	}
}

func TestDetectAPIPrefix_Unknown(t *testing.T) {
	root := &APIRootResponse{}
	got := DetectAPIPrefix(root)
	if got != "" {
		t.Errorf("DetectAPIPrefix = %q, want empty", got)
	}
}

func TestDetectAPIPrefix_Nil(t *testing.T) {
	got := DetectAPIPrefix(nil)
	if got != "" {
		t.Errorf("DetectAPIPrefix(nil) = %q, want empty", got)
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"1.2.3", "1.2.4", -1},
		{"1.2.4", "1.2.3", 1},
		{"23.4.0", "4.7.8", 1},
		{"4.7.8", "23.4.0", -1},
		{"1.0", "1.0.0", 0},
		{"1.0.1", "1.0", 1},
		{"1", "1.0.0", 0},
		{"2", "1.9.9", 1},
	}
	for _, tc := range tests {
		t.Run(tc.a+"_vs_"+tc.b, func(t *testing.T) {
			got := CompareVersions(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestVersionAtLeast(t *testing.T) {
	tests := []struct {
		version, min string
		want         bool
	}{
		{"23.4.0", "23.0.0", true},
		{"23.4.0", "23.4.0", true},
		{"23.4.0", "24.0.0", false},
		{"4.7.8", "4.7.0", true},
		{"4.7.8", "4.8.0", false},
		{"", "1.0.0", true},  // empty version = always true
		{"1.0.0", "", true},  // empty min = always true
		{"", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.version+"_gte_"+tc.min, func(t *testing.T) {
			got := VersionAtLeast(tc.version, tc.min)
			if got != tc.want {
				t.Errorf("VersionAtLeast(%q, %q) = %v, want %v", tc.version, tc.min, got, tc.want)
			}
		})
	}
}

func TestRewritePaths(t *testing.T) {
	resources := []models.ResourceType{
		{Name: "organizations", Label: "Organizations", APIPath: "/api/v2/organizations/", Skip: map[string]bool{"Default": true}},
		{Name: "teams", Label: "Teams", APIPath: "/api/v2/teams/"},
	}

	rewritten := rewritePaths(resources, "/api/v2/", "/api/v3/")

	if rewritten[0].APIPath != "/api/v3/organizations/" {
		t.Errorf("rewritten[0].APIPath = %q, want %q", rewritten[0].APIPath, "/api/v3/organizations/")
	}
	if rewritten[1].APIPath != "/api/v3/teams/" {
		t.Errorf("rewritten[1].APIPath = %q, want %q", rewritten[1].APIPath, "/api/v3/teams/")
	}

	// Verify original is unchanged
	if resources[0].APIPath != "/api/v2/organizations/" {
		t.Error("original resource was mutated")
	}

	// Verify Skip map is deeply copied
	rewritten[0].Skip["NewEntry"] = true
	if resources[0].Skip["NewEntry"] {
		t.Error("Skip map was not deeply copied")
	}
}

func TestRewritePaths_NoMatch(t *testing.T) {
	resources := []models.ResourceType{
		{Name: "orgs", APIPath: "/api/controller/v2/organizations/"},
	}
	rewritten := rewritePaths(resources, "/api/v2/", "/api/v3/")
	// No match, path stays the same
	if rewritten[0].APIPath != "/api/controller/v2/organizations/" {
		t.Errorf("rewritten[0].APIPath = %q, want unchanged", rewritten[0].APIPath)
	}
}

func TestPingWithVersion_Integration(t *testing.T) {
	pingResp := map[string]interface{}{
		"version":     "23.4.0",
		"ha":          false,
		"active_node": "awx-1",
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(pingResp)
	}))
	defer ts.Close()

	client := &Client{
		baseURL:    ts.URL,
		httpClient: ts.Client(),
	}

	resp, err := client.PingWithVersion("/api/v2/ping/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Version != "23.4.0" {
		t.Errorf("Version = %q, want %q", resp.Version, "23.4.0")
	}
}

func TestPingWithVersion_Unparseable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`)) // valid JSON but no version
	}))
	defer ts.Close()

	client := &Client{
		baseURL:    ts.URL,
		httpClient: ts.Client(),
	}

	resp, err := client.PingWithVersion("/api/v2/ping/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return empty version, not an error
	if resp.Version != "" {
		t.Errorf("Version = %q, want empty", resp.Version)
	}
}

func TestPingPath(t *testing.T) {
	tests := []struct {
		connType string
		want     string
	}{
		{"awx", "/api/v2/ping/"},
		{"aap", "/api/controller/v2/ping/"},
		{"", "/api/v2/ping/"},
	}
	for _, tc := range tests {
		t.Run(tc.connType, func(t *testing.T) {
			got := PingPath(tc.connType)
			if got != tc.want {
				t.Errorf("PingPath(%q) = %q, want %q", tc.connType, got, tc.want)
			}
		})
	}
}
