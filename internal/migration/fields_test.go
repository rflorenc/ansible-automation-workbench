package migration

import (
	"encoding/json"
	"testing"

	"github.com/rflorenc/ansible-automation-workbench/internal/models"
)

func TestToInt(t *testing.T) {
	tests := []struct {
		name   string
		input  interface{}
		expect int
	}{
		{"float64", float64(42), 42},
		{"int", 7, 7},
		{"json.Number", json.Number("99"), 99},
		{"nil", nil, 0},
		{"string", "not a number", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := toInt(tc.input)
			if got != tc.expect {
				t.Errorf("toInt(%v) = %d, want %d", tc.input, got, tc.expect)
			}
		})
	}
}

func TestStringField(t *testing.T) {
	obj := map[string]interface{}{
		"name":  "hello",
		"count": 42,
		"empty": nil,
	}
	if got := stringField(obj, "name"); got != "hello" {
		t.Errorf("stringField(name) = %q, want %q", got, "hello")
	}
	if got := stringField(obj, "count"); got != "" {
		t.Errorf("stringField(count) = %q, want empty", got)
	}
	if got := stringField(obj, "missing"); got != "" {
		t.Errorf("stringField(missing) = %q, want empty", got)
	}
}

func TestIntField(t *testing.T) {
	obj := map[string]interface{}{
		"id":   float64(10),
		"name": "test",
	}
	if got := intField(obj, "id"); got != 10 {
		t.Errorf("intField(id) = %d, want 10", got)
	}
	if got := intField(obj, "missing"); got != 0 {
		t.Errorf("intField(missing) = %d, want 0", got)
	}
}

func TestBoolField(t *testing.T) {
	obj := map[string]interface{}{
		"enabled":  true,
		"disabled": false,
		"name":     "test",
	}
	if got := boolField(obj, "enabled"); !got {
		t.Error("boolField(enabled) = false, want true")
	}
	if got := boolField(obj, "disabled"); got {
		t.Error("boolField(disabled) = true, want false")
	}
	if got := boolField(obj, "missing"); got {
		t.Error("boolField(missing) = true, want false")
	}
	if got := boolField(obj, "name"); got {
		t.Error("boolField(name) = true, want false (wrong type)")
	}
}

func TestSummaryField(t *testing.T) {
	r := models.Resource{
		"summary_fields": map[string]interface{}{
			"organization": map[string]interface{}{
				"name": "Default",
				"id":   float64(1),
			},
		},
	}

	if got, ok := summaryField(r, "organization", "name").(string); !ok || got != "Default" {
		t.Errorf("summaryField(organization, name) = %v, want Default", got)
	}
	if got := summaryField(r, "organization", "missing"); got != nil {
		t.Errorf("summaryField(organization, missing) = %v, want nil", got)
	}
	if got := summaryField(r, "nosection", "name"); got != nil {
		t.Errorf("summaryField(nosection, name) = %v, want nil", got)
	}

	// No summary_fields at all
	empty := models.Resource{"name": "test"}
	if got := summaryField(empty, "organization", "name"); got != nil {
		t.Errorf("summaryField on resource without summary_fields = %v, want nil", got)
	}
}

func TestExtractOrgName(t *testing.T) {
	r := models.Resource{
		"summary_fields": map[string]interface{}{
			"organization": map[string]interface{}{"name": "MyOrg"},
		},
	}
	if got := extractOrgName(r); got != "MyOrg" {
		t.Errorf("extractOrgName = %q, want MyOrg", got)
	}

	empty := models.Resource{}
	if got := extractOrgName(empty); got != "" {
		t.Errorf("extractOrgName(empty) = %q, want empty", got)
	}
}

func TestExtractCredentialNames(t *testing.T) {
	r := models.Resource{
		"summary_fields": map[string]interface{}{
			"credentials": []interface{}{
				map[string]interface{}{"name": "Machine", "id": float64(1)},
				map[string]interface{}{"name": "SCM", "id": float64(2)},
			},
		},
	}
	names := extractCredentialNames(r)
	if len(names) != 2 || names[0] != "Machine" || names[1] != "SCM" {
		t.Errorf("extractCredentialNames = %v, want [Machine SCM]", names)
	}

	// No credentials field
	empty := models.Resource{}
	if got := extractCredentialNames(empty); got != nil {
		t.Errorf("extractCredentialNames(empty) = %v, want nil", got)
	}

	// credentials is not an array
	bad := models.Resource{
		"summary_fields": map[string]interface{}{
			"credentials": "not-an-array",
		},
	}
	if got := extractCredentialNames(bad); got != nil {
		t.Errorf("extractCredentialNames(bad) = %v, want nil", got)
	}
}
