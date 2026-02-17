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
func NewPlatform(conn *models.Connection) Platform {
	client := NewClient(conn)
	switch conn.Type {
	case "awx":
		return NewAWXPlatform(client)
	case "aap":
		return NewAAPPlatform(client)
	default:
		return NewAWXPlatform(client)
	}
}
