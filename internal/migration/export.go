package migration

import (
	"fmt"

	"github.com/rflorenc/ansible-automation-workbench/internal/models"
	"github.com/rflorenc/ansible-automation-workbench/internal/platform"
)

// Default/system resource names to skip during export.
var skipNames = map[string]map[string]bool{
	"organizations": {"Default": true},
	"users":         {"admin": true},
	"credentials":   {"Demo Credential": true, "Ansible Galaxy": true},
	"projects":      {"Demo Project": true},
	"inventories":   {"Demo Inventory": true},
	"job_templates":  {"Demo Job Template": true},
}

// DefaultExclusions returns the default resource names skipped during migration export.
func DefaultExclusions() map[string][]string {
	result := make(map[string][]string)
	for typeName, names := range skipNames {
		for name := range names {
			result[typeName] = append(result[typeName], name)
		}
	}
	return result
}

// exportAll fetches all migratable resource types from the source into memory.
func exportAll(client *platform.Client, prefix string, logger func(string)) (*ExportedData, error) {
	data := &ExportedData{
		Hosts:         make(map[int][]models.Resource),
		Groups:        make(map[int][]models.Resource),
		GroupHosts:    make(map[int][]int),
		Surveys:       make(map[int]models.Resource),
		WorkflowNodes: make(map[int][]models.Resource),
		OrgUsers:      make(map[int][]string),
		TeamUsers:     make(map[int][]string),
	}

	var err error

	// 1. Organizations
	data.Organizations, err = fetchFiltered(client, prefix+"organizations/", "organizations", logger)
	if err != nil {
		return nil, err
	}

	// 2. Teams
	data.Teams, err = fetchFiltered(client, prefix+"teams/", "teams", logger)
	if err != nil {
		return nil, err
	}

	// 3. Users
	data.Users, err = fetchFiltered(client, prefix+"users/", "users", logger)
	if err != nil {
		return nil, err
	}

	// 4. Credential types (custom only — skip managed)
	logger("Exporting credential_types...")
	allCredTypes, err := client.GetAll(prefix + "credential_types/")
	if err != nil {
		return nil, fmt.Errorf("credential_types: %w", err)
	}
	for _, ct := range allCredTypes {
		if boolField(ct, "managed") {
			continue
		}
		data.CredentialTypes = append(data.CredentialTypes, ct)
	}
	logger(fmt.Sprintf("  %d custom credential types", len(data.CredentialTypes)))

	// 5. Credentials
	data.Credentials, err = fetchFiltered(client, prefix+"credentials/", "credentials", logger)
	if err != nil {
		return nil, err
	}

	// 6. Projects
	data.Projects, err = fetchFiltered(client, prefix+"projects/", "projects", logger)
	if err != nil {
		return nil, err
	}

	// 7. Inventories
	data.Inventories, err = fetchFiltered(client, prefix+"inventories/", "inventories", logger)
	if err != nil {
		return nil, err
	}

	// 8. Hosts and groups per inventory
	for _, inv := range data.Inventories {
		invID := resourceID(inv)
		invName := resourceName(inv)

		hosts, err := client.GetAll(fmt.Sprintf("%sinventories/%d/hosts/", prefix, invID))
		if err != nil {
			logger(fmt.Sprintf("  WARNING: failed to get hosts for inventory %s: %v", invName, err))
			continue
		}
		data.Hosts[invID] = hosts

		groups, err := client.GetAll(fmt.Sprintf("%sinventories/%d/groups/", prefix, invID))
		if err != nil {
			logger(fmt.Sprintf("  WARNING: failed to get groups for inventory %s: %v", invName, err))
			continue
		}
		data.Groups[invID] = groups

		// Group-host associations
		for _, g := range groups {
			gID := resourceID(g)
			gHosts, err := client.GetAll(fmt.Sprintf("%sgroups/%d/hosts/", prefix, gID))
			if err != nil {
				continue
			}
			for _, h := range gHosts {
				data.GroupHosts[gID] = append(data.GroupHosts[gID], resourceID(h))
			}
		}

		logger(fmt.Sprintf("  Inventory %s: %d hosts, %d groups", invName, len(hosts), len(groups)))
	}

	// 9. Job templates
	data.JobTemplates, err = fetchFiltered(client, prefix+"job_templates/", "job_templates", logger)
	if err != nil {
		return nil, err
	}

	// 10. Surveys for JTs
	for _, jt := range data.JobTemplates {
		if boolField(jt, "survey_enabled") {
			jtID := resourceID(jt)
			var survey models.Resource
			if err := client.GetJSON(fmt.Sprintf("%sjob_templates/%d/survey_spec/", prefix, jtID), nil, &survey); err == nil && survey != nil {
				data.Surveys[jtID] = survey
			}
		}
	}

	// 11. Workflow job templates
	data.WorkflowJTs, err = fetchFiltered(client, prefix+"workflow_job_templates/", "workflow_job_templates", logger)
	if err != nil {
		return nil, err
	}

	// 12. Workflow nodes and surveys
	for _, wf := range data.WorkflowJTs {
		wfID := resourceID(wf)
		wfName := resourceName(wf)

		nodes, err := client.GetAll(fmt.Sprintf("%sworkflow_job_templates/%d/workflow_nodes/", prefix, wfID))
		if err != nil {
			logger(fmt.Sprintf("  WARNING: failed to get nodes for workflow %s: %v", wfName, err))
			continue
		}
		data.WorkflowNodes[wfID] = nodes
		logger(fmt.Sprintf("  Workflow %s: %d nodes", wfName, len(nodes)))

		if boolField(wf, "survey_enabled") {
			var survey models.Resource
			if err := client.GetJSON(fmt.Sprintf("%sworkflow_job_templates/%d/survey_spec/", prefix, wfID), nil, &survey); err == nil && survey != nil {
				data.Surveys[wfID] = survey
			}
		}
	}

	// 13. Schedules (skip system-managed ones)
	logger("Exporting schedules...")
	allSchedules, err := client.GetAll(prefix + "schedules/")
	if err != nil {
		return nil, fmt.Errorf("schedules: %w", err)
	}
	// Build set of exported JT/WFJT names for schedule filtering
	exportedJTs := make(map[string]bool)
	for _, jt := range data.JobTemplates {
		exportedJTs[resourceName(jt)] = true
	}
	for _, wf := range data.WorkflowJTs {
		exportedJTs[resourceName(wf)] = true
	}
	for _, sched := range allSchedules {
		parentName := extractUnifiedJTName(sched)
		if parentName == "" || !exportedJTs[parentName] {
			continue
		}
		data.Schedules = append(data.Schedules, sched)
	}
	logger(fmt.Sprintf("  %d schedules", len(data.Schedules)))

	// 14. Org-user and team-user associations
	logger("Exporting user associations...")
	for _, org := range data.Organizations {
		orgID := resourceID(org)
		users, err := client.GetAll(fmt.Sprintf("%sorganizations/%d/users/", prefix, orgID))
		if err != nil {
			continue
		}
		for _, u := range users {
			username := stringField(u, "username")
			if username != "" && username != "admin" {
				data.OrgUsers[orgID] = append(data.OrgUsers[orgID], username)
			}
		}
	}
	for _, team := range data.Teams {
		teamID := resourceID(team)
		users, err := client.GetAll(fmt.Sprintf("%steams/%d/users/", prefix, teamID))
		if err != nil {
			continue
		}
		for _, u := range users {
			username := stringField(u, "username")
			if username != "" && username != "admin" {
				data.TeamUsers[teamID] = append(data.TeamUsers[teamID], username)
			}
		}
	}

	return data, nil
}

// fetchFiltered fetches all resources of a type and filters out defaults by name.
func fetchFiltered(client *platform.Client, path, typeName string, logger func(string)) ([]models.Resource, error) {
	logger(fmt.Sprintf("Exporting %s...", typeName))
	all, err := client.GetAll(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", typeName, err)
	}

	skip := skipNames[typeName]
	var filtered []models.Resource
	for _, r := range all {
		name := resourceName(r)
		if skip != nil && skip[name] {
			continue
		}
		filtered = append(filtered, r)
	}
	logger(fmt.Sprintf("  %d %s (skipped %d defaults)", len(filtered), typeName, len(all)-len(filtered)))
	return filtered, nil
}
