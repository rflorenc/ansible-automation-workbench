package migration

import (
	"fmt"

	"github.com/rflorenc/ansible-automation-workbench/internal/models"
	"github.com/rflorenc/ansible-automation-workbench/internal/platform"
)

// Resource types in the order they appear in the preview.
var previewOrder = []string{
	"organizations", "teams", "users", "credential_types", "credentials",
	"projects", "inventories", "job_templates", "workflow_job_templates", "schedules",
}

// preflightCheck examines the destination for each exported resource and classifies
// the action as "create" or "skip_exists".
func preflightCheck(data *ExportedData, dst *platform.Client, prefix string, logger func(string)) (*models.MigrationPreview, error) {
	preview := &models.MigrationPreview{
		Resources: make(map[string][]models.MigrationResource),
	}

	for _, rt := range previewOrder {
		items := dataForType(data, rt)
		if len(items) == 0 {
			continue
		}

		logger(fmt.Sprintf("Checking %s on destination...", rt))
		for _, item := range items {
			name := resourceName(item)
			srcID := resourceID(item)

			mr := models.MigrationResource{
				SourceID: srcID,
				Name:     name,
				Type:     rt,
			}

			var existing models.Resource
			var err error

			switch rt {
			case "users":
				existing, err = dst.FindByUsername(prefix+rt+"/", name)
			case "credential_types":
				existing, err = dst.FindByName(prefix+"credential_types/", name)
			default:
				existing, err = dst.FindByName(prefix+rt+"/", name)
			}

			if err == nil && existing != nil {
				mr.Action = "skip_exists"
				mr.DestID = resourceID(existing)
				logger(fmt.Sprintf("  %s: exists (dest ID %d)", name, mr.DestID))
			} else {
				mr.Action = "create"
			}

			preview.Resources[rt] = append(preview.Resources[rt], mr)
		}
	}

	// Warnings
	if len(data.Credentials) > 0 {
		preview.Warnings = append(preview.Warnings,
			"Credential secrets cannot be exported via API. Credentials will be created with empty inputs — you must set secrets manually after migration.")
	}
	if len(data.Users) > 0 {
		preview.Warnings = append(preview.Warnings,
			"User passwords cannot be exported. Users will be created with a placeholder password (changeme!) and must be reset.")
	}

	return preview, nil
}

// dataForType returns the exported resources for a given type name.
func dataForType(data *ExportedData, typeName string) []models.Resource {
	switch typeName {
	case "organizations":
		return data.Organizations
	case "teams":
		return data.Teams
	case "users":
		return data.Users
	case "credential_types":
		return data.CredentialTypes
	case "credentials":
		return data.Credentials
	case "projects":
		return data.Projects
	case "inventories":
		return data.Inventories
	case "job_templates":
		return data.JobTemplates
	case "workflow_job_templates":
		return data.WorkflowJTs
	case "schedules":
		return data.Schedules
	}
	return nil
}
