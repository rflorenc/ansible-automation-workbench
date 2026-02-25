package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rflorenc/ansible-automation-workbench/internal/models"
	"github.com/rflorenc/ansible-automation-workbench/internal/platform"
)

// idMap tracks source name → destination ID mappings for reference resolution.
type idMap struct {
	orgs         map[string]int
	teams        map[string]int
	users        map[string]int
	credTypes    map[string]int
	creds        map[string]int
	projects     map[string]int
	invs         map[string]int
	hosts        map[string]int // "invName/hostName" → dest ID
	groups       map[string]int // "invName/groupName" → dest ID
	jts          map[string]int
	wfjts        map[string]int
	credTypeByID map[int]int // source cred type ID → dest cred type ID
	nodes        map[int]int // source node ID → dest node ID
}

func newIDMap() *idMap {
	return &idMap{
		orgs:         make(map[string]int),
		teams:        make(map[string]int),
		users:        make(map[string]int),
		credTypes:    make(map[string]int),
		creds:        make(map[string]int),
		projects:     make(map[string]int),
		invs:         make(map[string]int),
		hosts:        make(map[string]int),
		groups:       make(map[string]int),
		jts:          make(map[string]int),
		wfjts:        make(map[string]int),
		credTypeByID: make(map[int]int),
		nodes:        make(map[int]int),
	}
}

// actionFor returns the preview action for a resource, defaulting to "create".
func actionFor(preview *models.MigrationPreview, typeName, name string) (string, int) {
	for _, mr := range preview.Resources[typeName] {
		if mr.Name == name {
			return mr.Action, mr.DestID
		}
	}
	return "create", 0
}

// isExcluded checks whether a resource should be excluded from migration.
func isExcluded(exclude map[string][]string, typeName, name string) bool {
	names, ok := exclude[typeName]
	if !ok {
		return false
	}
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// importAll creates resources on the destination in strict dependency order.
func importAll(ctx context.Context, dst *platform.Client, prefix, dstType string, data *ExportedData, preview *models.MigrationPreview, exclude map[string][]string, logger func(string)) error {
	if exclude == nil {
		exclude = make(map[string][]string)
	}
	ids := newIDMap()

	// Pre-populate credential type name→ID from destination (for both managed and custom types)
	allDestCT, _ := dst.GetAll(prefix + "credential_types/")
	for _, ct := range allDestCT {
		ids.credTypes[resourceName(ct)] = resourceID(ct)
	}

	// 1. Organizations
	if ctx.Err() != nil {
		logger("Migration cancelled by user")
		return ctx.Err()
	}
	logger("=== Importing organizations ===")
	for _, org := range data.Organizations {
		name := resourceName(org)
		if isExcluded(exclude, "organizations", name) {
			logger(fmt.Sprintf("  EXCLUDED: %s (user exclusion)", name))
			continue
		}
		action, destID := actionFor(preview, "organizations", name)
		if action != "create" {
			ids.orgs[name] = destID
			logger(fmt.Sprintf("  SKIP (exists): %s", name))
			continue
		}
		id, err := createResource(dst, prefix+"organizations/", map[string]interface{}{
			"name":        name,
			"description": stringField(org, "description"),
		})
		if err != nil {
			logger(fmt.Sprintf("  FAIL: %s: %v", name, err))
			continue
		}
		ids.orgs[name] = id
		logger(fmt.Sprintf("  CREATED: %s (ID %d)", name, id))
	}

	// 2. Credential types (custom only)
	if ctx.Err() != nil {
		logger("Migration cancelled by user")
		return ctx.Err()
	}
	logger("")
	logger("=== Importing credential types ===")
	for _, ct := range data.CredentialTypes {
		name := resourceName(ct)
		if isExcluded(exclude, "credential_types", name) {
			logger(fmt.Sprintf("  EXCLUDED: %s (user exclusion)", name))
			continue
		}
		action, destID := actionFor(preview, "credential_types", name)
		if action != "create" {
			ids.credTypes[name] = destID
			ids.credTypeByID[resourceID(ct)] = destID
			logger(fmt.Sprintf("  SKIP (exists): %s", name))
			continue
		}
		id, err := createResource(dst, prefix+"credential_types/", map[string]interface{}{
			"name":        name,
			"description": stringField(ct, "description"),
			"kind":        stringField(ct, "kind"),
			"inputs":      ct["inputs"],
			"injectors":   ct["injectors"],
		})
		if err != nil {
			logger(fmt.Sprintf("  FAIL: %s: %v", name, err))
			continue
		}
		ids.credTypes[name] = id
		ids.credTypeByID[resourceID(ct)] = id
		logger(fmt.Sprintf("  CREATED: %s (ID %d)", name, id))
	}

	// 3. Users
	if ctx.Err() != nil {
		logger("Migration cancelled by user")
		return ctx.Err()
	}
	logger("")
	logger("=== Importing users ===")
	for _, user := range data.Users {
		name := stringField(user, "username")
		if isExcluded(exclude, "users", name) {
			logger(fmt.Sprintf("  EXCLUDED: %s (user exclusion)", name))
			continue
		}
		action, destID := actionFor(preview, "users", name)
		if action != "create" {
			ids.users[name] = destID
			logger(fmt.Sprintf("  SKIP (exists): %s", name))
			continue
		}
		id, err := createResource(dst, prefix+"users/", map[string]interface{}{
			"username":     name,
			"first_name":   stringField(user, "first_name"),
			"last_name":    stringField(user, "last_name"),
			"email":        stringField(user, "email"),
			"is_superuser": false,
			"password":     "changeme!",
		})
		if err != nil {
			logger(fmt.Sprintf("  FAIL: %s: %v", name, err))
			continue
		}
		ids.users[name] = id
		logger(fmt.Sprintf("  CREATED: %s (ID %d)", name, id))
	}

	// 4. Teams
	if ctx.Err() != nil {
		logger("Migration cancelled by user")
		return ctx.Err()
	}
	logger("")
	logger("=== Importing teams ===")
	for _, team := range data.Teams {
		name := resourceName(team)
		if isExcluded(exclude, "teams", name) {
			logger(fmt.Sprintf("  EXCLUDED: %s (user exclusion)", name))
			continue
		}
		action, destID := actionFor(preview, "teams", name)
		if action != "create" {
			ids.teams[name] = destID
			logger(fmt.Sprintf("  SKIP (exists): %s", name))
			continue
		}
		orgName := extractOrgName(team)
		orgID := ids.orgs[orgName]
		if orgID == 0 {
			logger(fmt.Sprintf("  SKIP: %s (org %q not found)", name, orgName))
			continue
		}
		id, err := createResource(dst, prefix+"teams/", map[string]interface{}{
			"name":         name,
			"description":  stringField(team, "description"),
			"organization": orgID,
		})
		if err != nil {
			logger(fmt.Sprintf("  FAIL: %s: %v", name, err))
			continue
		}
		ids.teams[name] = id
		logger(fmt.Sprintf("  CREATED: %s (ID %d)", name, id))
	}

	// 5. Credentials
	if ctx.Err() != nil {
		logger("Migration cancelled by user")
		return ctx.Err()
	}
	logger("")
	logger("=== Importing credentials ===")
	for _, cred := range data.Credentials {
		name := resourceName(cred)
		if isExcluded(exclude, "credentials", name) {
			logger(fmt.Sprintf("  EXCLUDED: %s (user exclusion)", name))
			continue
		}
		action, destID := actionFor(preview, "credentials", name)
		if action != "create" {
			ids.creds[name] = destID
			logger(fmt.Sprintf("  SKIP (exists): %s", name))
			continue
		}
		orgName := extractOrgName(cred)
		orgID := ids.orgs[orgName]

		// Resolve credential type: try by source ID first, then by name
		srcCtID := intField(cred, "credential_type")
		destCtID := ids.credTypeByID[srcCtID]
		if destCtID == 0 {
			ctName := extractCredTypeName(cred)
			destCtID = ids.credTypes[ctName]
		}
		if destCtID == 0 {
			logger(fmt.Sprintf("  SKIP: %s (credential type not found)", name))
			continue
		}

		id, err := createResource(dst, prefix+"credentials/", map[string]interface{}{
			"name":            name,
			"description":     stringField(cred, "description"),
			"organization":    orgID,
			"credential_type": destCtID,
			"inputs":          map[string]interface{}{},
		})
		if err != nil {
			logger(fmt.Sprintf("  FAIL: %s: %v", name, err))
			continue
		}
		ids.creds[name] = id
		logger(fmt.Sprintf("  CREATED: %s (ID %d) [inputs empty — set secrets manually]", name, id))
	}

	// 6. Projects
	if ctx.Err() != nil {
		logger("Migration cancelled by user")
		return ctx.Err()
	}
	logger("")
	logger("=== Importing projects ===")
	var projectWaitList []struct {
		name string
		id   int
	}
	for _, proj := range data.Projects {
		name := resourceName(proj)
		if isExcluded(exclude, "projects", name) {
			logger(fmt.Sprintf("  EXCLUDED: %s (user exclusion)", name))
			continue
		}
		action, destID := actionFor(preview, "projects", name)
		if action != "create" {
			ids.projects[name] = destID
			logger(fmt.Sprintf("  SKIP (exists): %s", name))
			continue
		}
		orgName := extractOrgName(proj)
		orgID := ids.orgs[orgName]

		payload := map[string]interface{}{
			"name":                     name,
			"description":              stringField(proj, "description"),
			"organization":             orgID,
			"scm_type":                 stringField(proj, "scm_type"),
			"scm_url":                  stringField(proj, "scm_url"),
			"scm_branch":               stringField(proj, "scm_branch"),
			"scm_clean":                proj["scm_clean"],
			"scm_delete_on_update":     proj["scm_delete_on_update"],
			"scm_track_submodules":     proj["scm_track_submodules"],
			"scm_update_on_launch":     proj["scm_update_on_launch"],
			"scm_update_cache_timeout": proj["scm_update_cache_timeout"],
		}

		scmCredName := extractSCMCredName(proj)
		if scmCredName != "" {
			if scmCredID := ids.creds[scmCredName]; scmCredID != 0 {
				payload["credential"] = scmCredID
			}
		}

		id, err := createResource(dst, prefix+"projects/", payload)
		if err != nil {
			logger(fmt.Sprintf("  FAIL: %s: %v", name, err))
			continue
		}
		ids.projects[name] = id
		logger(fmt.Sprintf("  CREATED: %s (ID %d)", name, id))
		projectWaitList = append(projectWaitList, struct {
			name string
			id   int
		}{name, id})
	}

	// Wait for project syncs on AAP
	if dstType == "aap" && len(projectWaitList) > 0 {
		logger("  Waiting for project syncs...")
		for _, pw := range projectWaitList {
			if ctx.Err() != nil {
				logger("Migration cancelled by user")
				return ctx.Err()
			}
			if err := waitForProjectCtx(ctx, dst, prefix, pw.id, 120*time.Second); err != nil {
				logger(fmt.Sprintf("  WARNING: project %s sync: %v", pw.name, err))
			} else {
				logger(fmt.Sprintf("  Project %s sync complete", pw.name))
			}
		}
	}

	// 7. Inventories
	if ctx.Err() != nil {
		logger("Migration cancelled by user")
		return ctx.Err()
	}
	logger("")
	logger("=== Importing inventories ===")
	// Map source inv ID → name for host/group import
	srcInvNames := make(map[int]string)
	for _, inv := range data.Inventories {
		name := resourceName(inv)
		srcInvNames[resourceID(inv)] = name
		if isExcluded(exclude, "inventories", name) {
			logger(fmt.Sprintf("  EXCLUDED: %s (user exclusion)", name))
			continue
		}
		action, destID := actionFor(preview, "inventories", name)
		if action != "create" {
			ids.invs[name] = destID
			logger(fmt.Sprintf("  SKIP (exists): %s", name))
			continue
		}
		orgName := extractOrgName(inv)
		orgID := ids.orgs[orgName]
		id, err := createResource(dst, prefix+"inventories/", map[string]interface{}{
			"name":         name,
			"description":  stringField(inv, "description"),
			"organization": orgID,
			"variables":    stringField(inv, "variables"),
		})
		if err != nil {
			logger(fmt.Sprintf("  FAIL: %s: %v", name, err))
			continue
		}
		ids.invs[name] = id
		logger(fmt.Sprintf("  CREATED: %s (ID %d)", name, id))
	}

	// 8. Hosts per inventory
	if ctx.Err() != nil {
		logger("Migration cancelled by user")
		return ctx.Err()
	}
	logger("")
	logger("=== Importing hosts ===")
	srcHostNames := make(map[int]string) // source host ID → name
	for srcInvID, hosts := range data.Hosts {
		invName := srcInvNames[srcInvID]
		destInvID := ids.invs[invName]
		if destInvID == 0 {
			continue
		}
		// Skip hosts for excluded inventories
		if isExcluded(exclude, "inventories", invName) {
			logger(fmt.Sprintf("  EXCLUDED: %s (inventory excluded)", invName))
			for _, host := range hosts {
				srcHostNames[resourceID(host)] = resourceName(host)
			}
			continue
		}
		for _, host := range hosts {
			if ctx.Err() != nil {
				logger("Migration cancelled by user")
				return ctx.Err()
			}
			name := resourceName(host)
			srcHostNames[resourceID(host)] = name
			key := invName + "/" + name
			if isExcluded(exclude, "hosts", name) {
				logger(fmt.Sprintf("  EXCLUDED: %s/%s (user exclusion)", invName, name))
				continue
			}
			// Check if host already exists
			existing, _ := dst.FindByName(fmt.Sprintf("%sinventories/%d/hosts/", prefix, destInvID), name)
			if existing != nil {
				ids.hosts[key] = resourceID(existing)
				continue
			}
			id, err := createResource(dst, fmt.Sprintf("%sinventories/%d/hosts/", prefix, destInvID), map[string]interface{}{
				"name":        name,
				"description": stringField(host, "description"),
				"variables":   stringField(host, "variables"),
				"enabled":     host["enabled"],
			})
			if err != nil {
				logger(fmt.Sprintf("  FAIL: %s/%s: %v", invName, name, err))
				continue
			}
			ids.hosts[key] = id
		}
		logger(fmt.Sprintf("  %s: %d hosts", invName, len(hosts)))
	}

	// 9. Groups per inventory + host associations
	if ctx.Err() != nil {
		logger("Migration cancelled by user")
		return ctx.Err()
	}
	logger("")
	logger("=== Importing groups ===")
	for srcInvID, groups := range data.Groups {
		invName := srcInvNames[srcInvID]
		destInvID := ids.invs[invName]
		if destInvID == 0 {
			continue
		}
		// Skip groups for excluded inventories
		if isExcluded(exclude, "inventories", invName) {
			continue
		}
		for _, group := range groups {
			if ctx.Err() != nil {
				logger("Migration cancelled by user")
				return ctx.Err()
			}
			name := resourceName(group)
			key := invName + "/" + name
			srcGroupID := resourceID(group)

			existing, _ := dst.FindByName(fmt.Sprintf("%sinventories/%d/groups/", prefix, destInvID), name)
			var destGroupID int
			if existing != nil {
				destGroupID = resourceID(existing)
				ids.groups[key] = destGroupID
			} else {
				id, err := createResource(dst, fmt.Sprintf("%sinventories/%d/groups/", prefix, destInvID), map[string]interface{}{
					"name":        name,
					"description": stringField(group, "description"),
					"variables":   stringField(group, "variables"),
				})
				if err != nil {
					logger(fmt.Sprintf("  FAIL: %s/%s: %v", invName, name, err))
					continue
				}
				destGroupID = id
				ids.groups[key] = id
			}

			// Associate hosts to group
			for _, srcHostID := range data.GroupHosts[srcGroupID] {
				hostName := srcHostNames[srcHostID]
				hostKey := invName + "/" + hostName
				if destHostID, ok := ids.hosts[hostKey]; ok {
					dst.Post(fmt.Sprintf("%sgroups/%d/hosts/", prefix, destGroupID),
						map[string]interface{}{"id": destHostID})
				}
			}
		}
		logger(fmt.Sprintf("  %s: %d groups", invName, len(groups)))
	}

	// 10. Job templates
	if ctx.Err() != nil {
		logger("Migration cancelled by user")
		return ctx.Err()
	}
	logger("")
	logger("=== Importing job templates ===")
	for _, jt := range data.JobTemplates {
		name := resourceName(jt)
		if isExcluded(exclude, "job_templates", name) {
			logger(fmt.Sprintf("  EXCLUDED: %s (user exclusion)", name))
			continue
		}
		action, destID := actionFor(preview, "job_templates", name)
		if action != "create" {
			ids.jts[name] = destID
			logger(fmt.Sprintf("  SKIP (exists): %s", name))
			continue
		}

		projName := extractProjectName(jt)
		invName := extractInventoryName(jt)

		payload := map[string]interface{}{
			"name":                                 name,
			"description":                          stringField(jt, "description"),
			"job_type":                             stringField(jt, "job_type"),
			"playbook":                             stringField(jt, "playbook"),
			"forks":                                jt["forks"],
			"limit":                                stringField(jt, "limit"),
			"verbosity":                            jt["verbosity"],
			"extra_vars":                           stringField(jt, "extra_vars"),
			"ask_variables_on_launch":              jt["ask_variables_on_launch"],
			"ask_limit_on_launch":                  jt["ask_limit_on_launch"],
			"ask_tags_on_launch":                   jt["ask_tags_on_launch"],
			"ask_diff_mode_on_launch":              jt["ask_diff_mode_on_launch"],
			"ask_skip_tags_on_launch":              jt["ask_skip_tags_on_launch"],
			"ask_job_type_on_launch":               jt["ask_job_type_on_launch"],
			"ask_credential_on_launch":             jt["ask_credential_on_launch"],
			"ask_verbosity_on_launch":              jt["ask_verbosity_on_launch"],
			"ask_inventory_on_launch":              jt["ask_inventory_on_launch"],
			"ask_scm_branch_on_launch":             jt["ask_scm_branch_on_launch"],
			"ask_execution_environment_on_launch":  jt["ask_execution_environment_on_launch"],
			"ask_labels_on_launch":                 jt["ask_labels_on_launch"],
			"ask_forks_on_launch":                  jt["ask_forks_on_launch"],
			"ask_job_slice_count_on_launch":        jt["ask_job_slice_count_on_launch"],
			"ask_timeout_on_launch":                jt["ask_timeout_on_launch"],
			"survey_enabled":                       jt["survey_enabled"],
			"become_enabled":                       jt["become_enabled"],
			"diff_mode":                            jt["diff_mode"],
			"allow_simultaneous":                   jt["allow_simultaneous"],
			"job_slice_count":                      jt["job_slice_count"],
			"timeout":                              jt["timeout"],
			"use_fact_cache":                       jt["use_fact_cache"],
			"host_config_key":                      stringField(jt, "host_config_key"),
			"scm_branch":                           stringField(jt, "scm_branch"),
		}

		if projID := ids.projects[projName]; projID != 0 {
			payload["project"] = projID
		}
		if invID := ids.invs[invName]; invID != 0 {
			payload["inventory"] = invID
		}

		id, err := createResource(dst, prefix+"job_templates/", payload)
		if err != nil {
			logger(fmt.Sprintf("  FAIL: %s: %v", name, err))
			continue
		}
		ids.jts[name] = id
		logger(fmt.Sprintf("  CREATED: %s (ID %d)", name, id))

		// Associate credentials
		for _, credName := range extractCredentialNames(jt) {
			if credID := ids.creds[credName]; credID != 0 {
				dst.Post(fmt.Sprintf("%sjob_templates/%d/credentials/", prefix, id),
					map[string]interface{}{"id": credID})
			}
		}

		// Import survey
		srcJTID := resourceID(jt)
		if survey, ok := data.Surveys[srcJTID]; ok {
			dst.Post(fmt.Sprintf("%sjob_templates/%d/survey_spec/", prefix, id), survey)
		}
	}

	// 11. Schedules
	if ctx.Err() != nil {
		logger("Migration cancelled by user")
		return ctx.Err()
	}
	logger("")
	logger("=== Importing schedules ===")
	for _, sched := range data.Schedules {
		name := resourceName(sched)
		if isExcluded(exclude, "schedules", name) {
			logger(fmt.Sprintf("  EXCLUDED: %s (user exclusion)", name))
			continue
		}
		parentName := extractUnifiedJTName(sched)
		destParentID := ids.jts[parentName]
		if destParentID == 0 {
			destParentID = ids.wfjts[parentName]
		}
		if destParentID == 0 {
			logger(fmt.Sprintf("  SKIP: %s (parent %q not found)", name, parentName))
			continue
		}

		// Determine parent endpoint
		parentEndpoint := "job_templates"
		if ids.wfjts[parentName] != 0 {
			parentEndpoint = "workflow_job_templates"
		}

		_, err := createResource(dst, fmt.Sprintf("%s%s/%d/schedules/", prefix, parentEndpoint, destParentID), map[string]interface{}{
			"name":  name,
			"rrule": stringField(sched, "rrule"),
		})
		if err != nil {
			logger(fmt.Sprintf("  FAIL: %s: %v", name, err))
			continue
		}
		logger(fmt.Sprintf("  CREATED: %s", name))
	}

	// 12. Workflow job templates
	if ctx.Err() != nil {
		logger("Migration cancelled by user")
		return ctx.Err()
	}
	logger("")
	logger("=== Importing workflow job templates ===")
	for _, wf := range data.WorkflowJTs {
		name := resourceName(wf)
		if isExcluded(exclude, "workflow_job_templates", name) {
			logger(fmt.Sprintf("  EXCLUDED: %s (user exclusion)", name))
			continue
		}
		action, destID := actionFor(preview, "workflow_job_templates", name)
		if action != "create" {
			ids.wfjts[name] = destID
			logger(fmt.Sprintf("  SKIP (exists): %s", name))
			continue
		}
		orgName := extractOrgName(wf)
		orgID := ids.orgs[orgName]

		id, err := createResource(dst, prefix+"workflow_job_templates/", map[string]interface{}{
			"name":                     name,
			"description":              stringField(wf, "description"),
			"organization":             orgID,
			"survey_enabled":           wf["survey_enabled"],
			"allow_simultaneous":       wf["allow_simultaneous"],
			"ask_variables_on_launch":  wf["ask_variables_on_launch"],
			"ask_inventory_on_launch":  wf["ask_inventory_on_launch"],
			"ask_scm_branch_on_launch": wf["ask_scm_branch_on_launch"],
			"ask_limit_on_launch":      wf["ask_limit_on_launch"],
			"ask_labels_on_launch":     wf["ask_labels_on_launch"],
			"extra_vars":               stringField(wf, "extra_vars"),
			"limit":                    stringField(wf, "limit"),
			"scm_branch":              stringField(wf, "scm_branch"),
		})
		if err != nil {
			logger(fmt.Sprintf("  FAIL: %s: %v", name, err))
			continue
		}
		ids.wfjts[name] = id
		logger(fmt.Sprintf("  CREATED: %s (ID %d)", name, id))
	}

	// 13. Workflow nodes — two passes: create nodes, then wire edges
	if ctx.Err() != nil {
		logger("Migration cancelled by user")
		return ctx.Err()
	}
	logger("")
	logger("=== Importing workflow nodes ===")
	for _, wf := range data.WorkflowJTs {
		wfName := resourceName(wf)
		srcWFID := resourceID(wf)
		destWFID := ids.wfjts[wfName]
		if destWFID == 0 {
			continue
		}
		nodes := data.WorkflowNodes[srcWFID]
		if len(nodes) == 0 {
			continue
		}

		// Pass 1: create all nodes
		for _, node := range nodes {
			ujtName := extractUnifiedJTName(node)
			destUJTID := ids.jts[ujtName]
			if destUJTID == 0 {
				destUJTID = ids.wfjts[ujtName]
			}
			if destUJTID == 0 {
				logger(fmt.Sprintf("  SKIP node: unified_job_template %q not found", ujtName))
				continue
			}

			nodeID, err := createResource(dst,
				fmt.Sprintf("%sworkflow_job_templates/%d/workflow_nodes/", prefix, destWFID),
				map[string]interface{}{"unified_job_template": destUJTID})
			if err != nil {
				logger(fmt.Sprintf("  FAIL node for %s: %v", ujtName, err))
				continue
			}
			ids.nodes[resourceID(node)] = nodeID
		}

		// Pass 2: wire edges
		for _, node := range nodes {
			srcNodeID := resourceID(node)
			destNodeID := ids.nodes[srcNodeID]
			if destNodeID == 0 {
				continue
			}

			wireEdges(dst, prefix, destNodeID, node, "success_nodes", ids)
			wireEdges(dst, prefix, destNodeID, node, "failure_nodes", ids)
			wireEdges(dst, prefix, destNodeID, node, "always_nodes", ids)
		}

		logger(fmt.Sprintf("  Workflow %s: %d nodes", wfName, len(nodes)))

		// Import WFJT survey
		if survey, ok := data.Surveys[srcWFID]; ok {
			dst.Post(fmt.Sprintf("%sworkflow_job_templates/%d/survey_spec/", prefix, destWFID), survey)
		}
	}

	// 14. User-org associations
	if ctx.Err() != nil {
		logger("Migration cancelled by user")
		return ctx.Err()
	}
	logger("")
	logger("=== Importing user-org associations ===")
	for _, org := range data.Organizations {
		srcOrgID := resourceID(org)
		orgName := resourceName(org)
		destOrgID := ids.orgs[orgName]
		if destOrgID == 0 {
			continue
		}
		for _, username := range data.OrgUsers[srcOrgID] {
			if destUserID := ids.users[username]; destUserID != 0 {
				dst.Post(fmt.Sprintf("%sorganizations/%d/users/", prefix, destOrgID),
					map[string]interface{}{"id": destUserID})
			}
		}
		if len(data.OrgUsers[srcOrgID]) > 0 {
			logger(fmt.Sprintf("  %s: %d users", orgName, len(data.OrgUsers[srcOrgID])))
		}
	}

	// 15. User-team associations
	logger("=== Importing user-team associations ===")
	for _, team := range data.Teams {
		srcTeamID := resourceID(team)
		teamName := resourceName(team)
		destTeamID := ids.teams[teamName]
		if destTeamID == 0 {
			continue
		}
		for _, username := range data.TeamUsers[srcTeamID] {
			if destUserID := ids.users[username]; destUserID != 0 {
				dst.Post(fmt.Sprintf("%steams/%d/users/", prefix, destTeamID),
					map[string]interface{}{"id": destUserID})
			}
		}
		if len(data.TeamUsers[srcTeamID]) > 0 {
			logger(fmt.Sprintf("  %s: %d users", teamName, len(data.TeamUsers[srcTeamID])))
		}
	}

	logger("")
	logger("=== Migration complete ===")
	return nil
}

// createResource POSTs a payload and returns the new resource ID.
func createResource(client *platform.Client, path string, payload map[string]interface{}) (int, error) {
	body, statusCode, err := client.Post(path, payload)
	if err != nil {
		return 0, err
	}
	_ = statusCode

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("parsing response: %w", err)
	}
	return toInt(result["id"]), nil
}

// wireEdges connects workflow node edges (success_nodes, failure_nodes, always_nodes).
func wireEdges(dst *platform.Client, prefix string, destNodeID int, node models.Resource, edgeType string, ids *idMap) {
	edges, ok := node[edgeType].([]interface{})
	if !ok {
		return
	}
	for _, e := range edges {
		srcTargetID := int(0)
		switch v := e.(type) {
		case float64:
			srcTargetID = int(v)
		case json.Number:
			i, _ := v.Int64()
			srcTargetID = int(i)
		}
		if destTargetID := ids.nodes[srcTargetID]; destTargetID != 0 {
			dst.Post(fmt.Sprintf("%sworkflow_job_template_nodes/%d/%s/", prefix, destNodeID, edgeType),
				map[string]interface{}{"id": destTargetID})
		}
	}
}

// waitForProject polls project status until sync completes or times out.
func waitForProject(client *platform.Client, prefix string, id int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var proj map[string]interface{}
		err := client.GetJSON(fmt.Sprintf("%sprojects/%d/", prefix, id), nil, &proj)
		if err != nil {
			return err
		}
		status, _ := proj["status"].(string)
		switch status {
		case "successful":
			return nil
		case "failed", "error", "canceled":
			return fmt.Errorf("project sync status: %s", status)
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("timeout waiting for project sync")
}

// waitForProjectCtx is like waitForProject but respects context cancellation.
func waitForProjectCtx(ctx context.Context, client *platform.Client, prefix string, id int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var proj map[string]interface{}
		err := client.GetJSON(fmt.Sprintf("%sprojects/%d/", prefix, id), nil, &proj)
		if err != nil {
			return err
		}
		status, _ := proj["status"].(string)
		switch status {
		case "successful":
			return nil
		case "failed", "error", "canceled":
			return fmt.Errorf("project sync status: %s", status)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
	return fmt.Errorf("timeout waiting for project sync")
}
