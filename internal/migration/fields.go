package migration

import (
	"encoding/json"

	"github.com/rflorenc/ansible-automation-workbench/internal/models"
)

// resourceID extracts the numeric ID from a Resource.
func resourceID(r models.Resource) int {
	return toInt(r["id"])
}

// resourceName returns the name (or username) of a Resource.
func resourceName(r models.Resource) string {
	if n, ok := r["name"].(string); ok {
		return n
	}
	if u, ok := r["username"].(string); ok {
		return u
	}
	return ""
}

// intField safely extracts an int field from a map.
func intField(obj map[string]interface{}, field string) int {
	return toInt(obj[field])
}

// stringField safely extracts a string field, returning "" if nil.
func stringField(obj map[string]interface{}, field string) string {
	if v, ok := obj[field].(string); ok {
		return v
	}
	return ""
}

// boolField safely extracts a bool field, returning false if nil.
func boolField(obj map[string]interface{}, field string) bool {
	if v, ok := obj[field].(bool); ok {
		return v
	}
	return false
}

// summaryField navigates summary_fields.{section}.{field}.
func summaryField(r models.Resource, section, field string) interface{} {
	sf, ok := r["summary_fields"].(map[string]interface{})
	if !ok {
		return nil
	}
	sec, ok := sf[section].(map[string]interface{})
	if !ok {
		return nil
	}
	return sec[field]
}

// extractOrgName returns summary_fields.organization.name.
func extractOrgName(r models.Resource) string {
	if v, ok := summaryField(r, "organization", "name").(string); ok {
		return v
	}
	return ""
}

// extractProjectName returns summary_fields.project.name.
func extractProjectName(r models.Resource) string {
	if v, ok := summaryField(r, "project", "name").(string); ok {
		return v
	}
	return ""
}

// extractInventoryName returns summary_fields.inventory.name.
func extractInventoryName(r models.Resource) string {
	if v, ok := summaryField(r, "inventory", "name").(string); ok {
		return v
	}
	return ""
}

// extractCredTypeName returns summary_fields.credential_type.name.
func extractCredTypeName(r models.Resource) string {
	if v, ok := summaryField(r, "credential_type", "name").(string); ok {
		return v
	}
	return ""
}

// extractSCMCredName returns summary_fields.credential.name (SCM credential on projects).
func extractSCMCredName(r models.Resource) string {
	if v, ok := summaryField(r, "credential", "name").(string); ok {
		return v
	}
	return ""
}

// extractUnifiedJTName returns summary_fields.unified_job_template.name.
func extractUnifiedJTName(r models.Resource) string {
	if v, ok := summaryField(r, "unified_job_template", "name").(string); ok {
		return v
	}
	return ""
}

// extractCredentialNames returns names from summary_fields.credentials[].name.
func extractCredentialNames(r models.Resource) []string {
	sf, ok := r["summary_fields"].(map[string]interface{})
	if !ok {
		return nil
	}
	creds, ok := sf["credentials"].([]interface{})
	if !ok {
		return nil
	}
	var names []string
	for _, c := range creds {
		if cm, ok := c.(map[string]interface{}); ok {
			if name, ok := cm["name"].(string); ok {
				names = append(names, name)
			}
		}
	}
	return names
}

// toInt converts various numeric types to int.
func toInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}
