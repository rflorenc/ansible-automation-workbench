package platform

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/rflorenc/ansible-automation-workbench/internal/models"
)

// PingResponse holds the parsed /ping/ response.
type PingResponse struct {
	Version string `json:"version"`
}

// APIRootResponse holds the parsed /api/ response.
// AWX format: {"current_version": "/api/v2/", ...}
// AAP format: {"apis": {"controller": "/api/controller/", ...}}
type APIRootResponse struct {
	CurrentVersion string            `json:"current_version"` // AWX
	APIs           map[string]string `json:"apis"`            // AAP: service name → prefix path
}

// ParsePingResponse extracts the version from a /ping/ JSON response body.
func ParsePingResponse(body []byte) (*PingResponse, error) {
	var resp PingResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing ping response: %w", err)
	}
	if resp.Version == "" {
		return nil, fmt.Errorf("ping response missing version field")
	}
	return &resp, nil
}

// ParseAPIRoot parses the /api/ response body.
func ParseAPIRoot(body []byte) (*APIRootResponse, error) {
	var resp APIRootResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing API root response: %w", err)
	}
	return &resp, nil
}

// DetectAPIPrefix determines the API prefix from the parsed /api/ response.
// AWX: uses current_version directly (e.g. "/api/v2/").
// AAP: uses apis.controller.prefix + "v2/" (e.g. "/api/controller/" → "/api/controller/v2/").
// Returns empty string if detection fails.
func DetectAPIPrefix(root *APIRootResponse) string {
	if root == nil {
		return ""
	}
	// AWX format: current_version is set
	if root.CurrentVersion != "" {
		prefix := root.CurrentVersion
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		return prefix
	}
	// AAP format: look for controller in apis
	if controllerPrefix, ok := root.APIs["controller"]; ok && controllerPrefix != "" {
		prefix := controllerPrefix
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		return prefix + "v2/"
	}
	return ""
}

// CompareVersions performs a simple semver comparison.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
// Handles partial versions (e.g. "4.7" vs "4.7.8").
func CompareVersions(a, b string) int {
	aParts := parseVersionParts(a)
	bParts := parseVersionParts(b)

	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		var av, bv int
		if i < len(aParts) {
			av = aParts[i]
		}
		if i < len(bParts) {
			bv = bParts[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

// VersionAtLeast returns true if version >= min.
func VersionAtLeast(version, min string) bool {
	if version == "" || min == "" {
		return true
	}
	return CompareVersions(version, min) >= 0
}

func parseVersionParts(v string) []int {
	parts := strings.Split(v, ".")
	result := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			break
		}
		result = append(result, n)
	}
	return result
}

// PingPaths returns the ping endpoint paths to try for a connection type.
// AAP tries the gateway path first, then falls back to the non-gateway path
// (AAP 2.4 RPM has no gateway and uses /api/v2/).
func PingPaths(connType string) []string {
	if connType == "aap" {
		return []string{"/api/controller/v2/ping/", "/api/v2/ping/"}
	}
	return []string{"/api/v2/ping/"}
}

// rewritePaths returns a copy of the resource registry with API paths rewritten
// from oldPrefix to newPrefix.
func rewritePaths(resources []models.ResourceType, oldPrefix, newPrefix string) []models.ResourceType {
	result := make([]models.ResourceType, len(resources))
	for i, r := range resources {
		result[i] = r
		result[i].APIPath = strings.Replace(r.APIPath, oldPrefix, newPrefix, 1)
		// Copy the Skip map to avoid sharing state
		if r.Skip != nil {
			skip := make(map[string]bool, len(r.Skip))
			for k, v := range r.Skip {
				skip[k] = v
			}
			result[i].Skip = skip
		}
	}
	return result
}

// PingWithVersion calls the ping endpoint using an authenticated client and
// parses the version from the response. If the response can't be parsed but
// HTTP succeeded, returns an empty PingResponse (connectivity OK, version unknown).
func (c *Client) PingWithVersion(apiPath string) (*PingResponse, error) {
	body, err := c.Get(apiPath, nil)
	if err != nil {
		return nil, err
	}
	resp, err := ParsePingResponse(body)
	if err != nil {
		// HTTP succeeded but couldn't parse version — not fatal
		return &PingResponse{}, nil
	}
	return resp, nil
}

// DiscoverAndStore orchestrates API discovery for a connection.
// It calls /api/ to detect the API prefix, then stores the result on the connection.
// All discovery is best-effort: failures are logged but do not produce errors.
func DiscoverAndStore(client *Client, conn *models.Connection, store *models.ConnectionStore) {
	// GET /api/ to discover prefix
	body, err := client.Get("/api/", nil)
	if err != nil {
		log.Printf("  DISCOVERY: %s: /api/ failed: %v", conn.Name, err)
		return
	}

	root, err := ParseAPIRoot(body)
	if err != nil {
		log.Printf("  DISCOVERY: %s: parse /api/ failed: %v", conn.Name, err)
		return
	}

	prefix := DetectAPIPrefix(root)
	if prefix == "" {
		log.Printf("  DISCOVERY: %s: could not detect API prefix", conn.Name)
		return
	}

	store.SetVersion(conn.ID, conn.Version, prefix)
	// Update local conn so callers see it immediately
	conn.APIPrefix = prefix
	fmt.Printf("  DISCOVERY: %s: detected API prefix: %s\n", conn.Name, prefix)
}
