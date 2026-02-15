package models

// Resource represents a generic API resource (org, team, credential, etc.).
type Resource map[string]interface{}

// ResourceType describes a browsable resource type on a platform.
type ResourceType struct {
	Name    string            `json:"name"`     // "organizations", "job_templates", etc.
	Label   string            `json:"label"`    // Human-readable: "Job Templates"
	APIPath string            `json:"api_path"` // "/api/v2/job_templates/"
	Skip    map[string]bool   `json:"-"`        // Names to never delete
}
