package platform

import "github.com/rflorenc/ansible-automation-workbench/internal/models"

// Platform defines operations available on an automation platform (AWX or AAP).
type Platform interface {
	// Ping tests connectivity (unauthenticated). Returns nil if reachable.
	Ping() error

	// CheckAuth verifies credentials. Returns nil if authenticated.
	CheckAuth() error

	// ListResources returns all objects of a given resource type.
	ListResources(resourceType string) ([]models.Resource, error)

	// GetResourceTypes returns all browsable resource types for this platform.
	GetResourceTypes() []models.ResourceType

	// Cleanup deletes non-default objects in correct dependency order.
	Cleanup(logger func(string)) error

	// Populate creates sample objects (AWX only).
	Populate(logger func(string)) error

	// Export downloads assets in breadth-first dependency order (AAP only).
	Export(outputDir string, logger func(string)) error
}

// NewPlatform creates the appropriate Platform implementation for a connection.
// If the connection has a detected APIPrefix that differs from the default,
// resource paths are rewritten accordingly. No HTTP calls are made here.
func NewPlatform(conn *models.Connection) Platform {
	client := NewClient(conn)
	switch conn.Type {
	case "awx":
		p := NewAWXPlatform(client)
		p.version = conn.Version
		if conn.APIPrefix != "" && conn.APIPrefix != "/api/v2/" {
			p.resources = rewritePaths(awxResources, "/api/v2/", conn.APIPrefix)
		}
		return p
	case "aap":
		p := NewAAPPlatform(client)
		p.version = conn.Version
		if conn.APIPrefix != "" && conn.APIPrefix != "/api/controller/v2/" {
			p.resources = rewritePaths(aapResources, "/api/controller/v2/", conn.APIPrefix)
		}
		return p
	default:
		p := NewAWXPlatform(client)
		p.version = conn.Version
		return p
	}
}
