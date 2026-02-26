package models

// MigrationResource describes a single object being considered for migration.
type MigrationResource struct {
	SourceID int    `json:"source_id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Action   string `json:"action"` // "create", "skip_exists", "skip_default", "skip_managed"
	DestID   int    `json:"dest_id,omitempty"`
}

// MigrationPreview holds the results of the export + preflight check.
type MigrationPreview struct {
	SourceID      string                         `json:"source_id"`
	DestinationID string                         `json:"destination_id"`
	Resources     map[string][]MigrationResource `json:"resources"`
	Warnings      []string                       `json:"warnings"`
	HostCounts    map[string]int                 `json:"host_counts,omitempty"`  // inventory name → host count
	GroupCounts   map[string]int                 `json:"group_counts,omitempty"` // inventory name → group count
}
