package migration

import (
	"fmt"

	"github.com/rflorenc/ansible-automation-workbench/internal/models"
	"github.com/rflorenc/ansible-automation-workbench/internal/platform"
)

// ExportedData holds all resources fetched from the source, in memory.
type ExportedData struct {
	Organizations   []models.Resource
	Teams           []models.Resource
	Users           []models.Resource
	CredentialTypes []models.Resource
	Credentials     []models.Resource
	Projects        []models.Resource
	Inventories     []models.Resource
	Hosts           map[int][]models.Resource // inventory source ID → hosts
	Groups          map[int][]models.Resource // inventory source ID → groups
	GroupHosts      map[int][]int             // group source ID → host source IDs
	JobTemplates    []models.Resource
	Surveys         map[int]models.Resource   // JT/WFJT source ID → survey spec
	WorkflowJTs     []models.Resource
	WorkflowNodes   map[int][]models.Resource // WFJT source ID → nodes
	Schedules       []models.Resource
	OrgUsers        map[int][]string // org source ID → usernames
	TeamUsers       map[int][]string // team source ID → usernames
}

// apiPrefix returns the API path prefix for a connection type.
func apiPrefix(connType string) string {
	if connType == "aap" {
		return "/api/controller/v2/"
	}
	return "/api/v2/"
}

// Preview exports resources from source and checks the destination for conflicts.
// Returns the preview (for the UI) and the exported data (for the import step).
func Preview(src, dst *models.Connection, logger func(string)) (*models.MigrationPreview, *ExportedData, error) {
	srcClient := platform.NewClient(src)
	dstClient := platform.NewClient(dst)

	// Verify connectivity
	logger("Checking source connectivity...")
	srcPrefix := apiPrefix(src.Type)
	if _, err := srcClient.Get(srcPrefix+"organizations/", nil); err != nil {
		return nil, nil, fmt.Errorf("source connection failed: %w", err)
	}
	logger("Source OK: " + src.Name)

	logger("Checking destination connectivity...")
	dstPrefix := apiPrefix(dst.Type)
	if _, err := dstClient.Get(dstPrefix+"organizations/", nil); err != nil {
		return nil, nil, fmt.Errorf("destination connection failed: %w", err)
	}
	logger("Destination OK: " + dst.Name)

	// Export from source
	logger("")
	logger("=== Exporting from source ===")
	data, err := exportAll(srcClient, srcPrefix, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("export failed: %w", err)
	}

	// Preflight check on destination
	logger("")
	logger("=== Checking destination ===")
	preview, err := preflightCheck(data, dstClient, dstPrefix, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("preflight failed: %w", err)
	}

	preview.SourceID = src.ID
	preview.DestinationID = dst.ID

	// Summary
	var createCount, skipCount int
	for _, items := range preview.Resources {
		for _, item := range items {
			if item.Action == "create" {
				createCount++
			} else {
				skipCount++
			}
		}
	}
	logger("")
	logger(fmt.Sprintf("Preview complete: %d to create, %d to skip", createCount, skipCount))

	return preview, data, nil
}

// Run imports the previously exported data into the destination.
func Run(dst *models.Connection, data *ExportedData, preview *models.MigrationPreview, logger func(string)) error {
	dstClient := platform.NewClient(dst)
	dstPrefix := apiPrefix(dst.Type)

	logger("=== Starting migration to " + dst.Name + " ===")
	logger("")

	return importAll(dstClient, dstPrefix, dst.Type, data, preview, logger)
}
