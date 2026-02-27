package platform

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rflorenc/ansible-automation-workbench/internal/models"
)

// AAP resource types (registry).
var aapResources = []models.ResourceType{
	{Name: "organizations", Label: "Organizations", APIPath: "/api/controller/v2/organizations/",
		Skip: map[string]bool{"Default": true}},
	{Name: "teams", Label: "Teams", APIPath: "/api/controller/v2/teams/"},
	{Name: "users", Label: "Users", APIPath: "/api/controller/v2/users/",
		Skip: map[string]bool{"admin": true}},
	{Name: "credential_types", Label: "Credential Types", APIPath: "/api/controller/v2/credential_types/"},
	{Name: "credentials", Label: "Credentials", APIPath: "/api/controller/v2/credentials/",
		Skip: map[string]bool{"Demo Credential": true}},
	{Name: "projects", Label: "Projects", APIPath: "/api/controller/v2/projects/",
		Skip: map[string]bool{"Demo Project": true}},
	{Name: "inventories", Label: "Inventories", APIPath: "/api/controller/v2/inventories/",
		Skip: map[string]bool{"Demo Inventory": true}},
	{Name: "execution_environments", Label: "Execution Environments", APIPath: "/api/controller/v2/execution_environments/",
		Skip: map[string]bool{
			"Control Plane Execution Environment":  true,
			"Default execution environment":        true,
			"Ansible Engine 2.9 Execution Environment": true,
			"Minimal execution environment":        true,
		}},
	{Name: "job_templates", Label: "Job Templates", APIPath: "/api/controller/v2/job_templates/"},
	{Name: "workflow_job_templates", Label: "Workflows", APIPath: "/api/controller/v2/workflow_job_templates/"},
	{Name: "schedules", Label: "Schedules", APIPath: "/api/controller/v2/schedules/"},
}

// AAPPlatform implements Platform for AAP 2.x (controller + gateway).
type AAPPlatform struct {
	client    *Client
	resources []models.ResourceType // nil = use static aapResources
	version   string                // detected platform version
}

// NewAAPPlatform creates a new AAP Platform.
func NewAAPPlatform(client *Client) *AAPPlatform {
	return &AAPPlatform{client: client}
}

func (p *AAPPlatform) Ping() error {
	// Try gateway path first (AAP 2.5+), fall back to non-gateway (AAP 2.4 RPM)
	for _, path := range PingPaths("aap") {
		if err := p.client.Ping(path); err == nil {
			return nil
		}
	}
	return p.client.Ping("/api/v2/ping/")
}

func (p *AAPPlatform) CheckAuth() error {
	// Try gateway path first, fall back to non-gateway
	err := p.client.Ping("/api/controller/v2/organizations/?page_size=1")
	if err != nil {
		err = p.client.Ping("/api/v2/organizations/?page_size=1")
	}
	return err
}

func (p *AAPPlatform) GetResourceTypes() []models.ResourceType {
	registry := aapResources
	if p.resources != nil {
		registry = p.resources
	}
	if p.version == "" {
		return registry
	}
	var filtered []models.ResourceType
	for _, r := range registry {
		if VersionAtLeast(p.version, r.MinVersion) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func (p *AAPPlatform) ListResources(resourceType string) ([]models.Resource, error) {
	for _, rt := range p.GetResourceTypes() {
		if rt.Name == resourceType {
			return p.client.GetAll(rt.APIPath)
		}
	}
	return nil, fmt.Errorf("unknown resource type: %s", resourceType)
}

// Populate creates sample AAP objects (orgs, teams, users, creds, projects, inventories, JTs, workflows, RBAC).
func (p *AAPPlatform) Populate(logger func(string)) error {
	log := logger
	c := p.client

	// Helper: ensure resource exists (find by name, create if missing)
	ensure := func(path, name string, payload map[string]interface{}) (int, error) {
		res, err := c.FindByName(path, name)
		if err != nil {
			return 0, err
		}
		if res != nil {
			return resourceID(res), nil
		}
		body, _, err := c.Post(path, payload)
		if err != nil {
			return 0, fmt.Errorf("creating %s: %w", name, err)
		}
		var created models.Resource
		if err := json.Unmarshal(body, &created); err != nil {
			return 0, err
		}
		return resourceID(created), nil
	}

	ensureUser := func(path, username string, payload map[string]interface{}) (int, error) {
		res, err := c.FindByUsername(path, username)
		if err != nil {
			return 0, err
		}
		if res != nil {
			return resourceID(res), nil
		}
		body, _, err := c.Post(path, payload)
		if err != nil {
			return 0, fmt.Errorf("creating user %s: %w", username, err)
		}
		var created models.Resource
		if err := json.Unmarshal(body, &created); err != nil {
			return 0, err
		}
		return resourceID(created), nil
	}

	// Associate (POST with {"id": ...}, ignore errors for already-exists)
	associate := func(path string, id int) {
		c.Post(path, map[string]interface{}{"id": id})
	}

	// 1. Organizations
	log("\n=== Creating Organizations ===")
	orgCorpID, err := ensure("/api/controller/v2/organizations/", "MigrateMe-Corp", map[string]interface{}{
		"name": "MigrateMe-Corp", "description": "Primary corporation for migration testing",
	})
	if err != nil {
		return fmt.Errorf("org MigrateMe-Corp: %w", err)
	}
	log(fmt.Sprintf("  Organization: MigrateMe-Corp (id=%d)", orgCorpID))

	orgOpsID, err := ensure("/api/controller/v2/organizations/", "MigrateMe-Ops", map[string]interface{}{
		"name": "MigrateMe-Ops", "description": "Operations team organization",
	})
	if err != nil {
		return fmt.Errorf("org MigrateMe-Ops: %w", err)
	}
	log(fmt.Sprintf("  Organization: MigrateMe-Ops (id=%d)", orgOpsID))

	// 2. Teams
	log("\n=== Creating Teams ===")
	type teamDef struct {
		name  string
		orgID int
	}
	teams := []teamDef{
		{"DevOps", orgCorpID}, {"DBA", orgCorpID}, {"Security", orgCorpID},
		{"App Development", orgCorpID}, {"Network Operations", orgOpsID}, {"Infrastructure", orgOpsID},
	}
	teamIDs := make(map[string]int)
	for _, t := range teams {
		id, err := ensure("/api/controller/v2/teams/", t.name, map[string]interface{}{
			"name": t.name, "organization": t.orgID,
		})
		if err != nil {
			return fmt.Errorf("team %s: %w", t.name, err)
		}
		teamIDs[t.name] = id
		log(fmt.Sprintf("  Team: %s (id=%d)", t.name, id))
	}

	// 3. Users
	log("\n=== Creating Users ===")
	type userDef struct {
		username  string
		firstName string
		lastName  string
		email     string
		orgName   string
		teamNames []string
	}
	users := []userDef{
		{"jsmith", "John", "Smith", "jsmith@migrateme.com", "MigrateMe-Corp", []string{"DevOps"}},
		{"tchen", "Tina", "Chen", "tchen@migrateme.com", "MigrateMe-Corp", []string{"DevOps", "Security"}},
		{"jdoe", "Jane", "Doe", "jdoe@migrateme.com", "MigrateMe-Corp", []string{"DBA"}},
		{"nmiller", "Nick", "Miller", "nmiller@migrateme.com", "MigrateMe-Corp", []string{"DBA"}},
		{"mbrown", "Maria", "Brown", "mbrown@migrateme.com", "MigrateMe-Corp", []string{"Security"}},
		{"lpatel", "Liam", "Patel", "lpatel@migrateme.com", "MigrateMe-Corp", []string{"App Development"}},
		{"agarcia", "Ana", "Garcia", "agarcia@migrateme.com", "MigrateMe-Corp", []string{"App Development"}},
		{"dwang", "David", "Wang", "dwang@migrateme.com", "MigrateMe-Ops", []string{"Network Operations"}},
		{"swilson", "Sarah", "Wilson", "swilson@migrateme.com", "MigrateMe-Ops", []string{"Infrastructure"}},
		{"rkumar", "Raj", "Kumar", "rkumar@migrateme.com", "MigrateMe-Ops", []string{"Network Operations", "Infrastructure"}},
	}
	userIDs := make(map[string]int)
	orgNameToID := map[string]int{"MigrateMe-Corp": orgCorpID, "MigrateMe-Ops": orgOpsID}
	for _, u := range users {
		id, err := ensureUser("/api/controller/v2/users/", u.username, map[string]interface{}{
			"username": u.username, "first_name": u.firstName, "last_name": u.lastName,
			"email": u.email, "password": "changeme123!",
		})
		if err != nil {
			return fmt.Errorf("user %s: %w", u.username, err)
		}
		userIDs[u.username] = id
		log(fmt.Sprintf("  User: %s (id=%d)", u.username, id))

		// Associate with org
		associate(fmt.Sprintf("/api/controller/v2/organizations/%d/users/", orgNameToID[u.orgName]), id)
		// Associate with teams
		for _, tn := range u.teamNames {
			if tid, ok := teamIDs[tn]; ok {
				associate(fmt.Sprintf("/api/controller/v2/teams/%d/users/", tid), id)
			}
		}
	}

	// 4. Credential Types
	log("\n=== Creating Credential Types ===")
	ctID, err := ensure("/api/controller/v2/credential_types/", "API Token", map[string]interface{}{
		"name": "API Token",
		"kind": "cloud",
		"inputs": map[string]interface{}{
			"fields": []map[string]interface{}{
				{"id": "api_url", "type": "string", "label": "API URL"},
				{"id": "api_token", "type": "string", "label": "API Token", "secret": true},
			},
			"required": []string{"api_url", "api_token"},
		},
		"injectors": map[string]interface{}{
			"extra_vars": map[string]interface{}{
				"api_url":   "{{ api_url }}",
				"api_token": "{{ api_token }}",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("credential type: %w", err)
	}
	log(fmt.Sprintf("  Credential Type: API Token (id=%d)", ctID))

	// 5. Credentials
	log("\n=== Creating Credentials ===")
	type credDef struct {
		name     string
		credType int
		orgID    int
		inputs   map[string]interface{}
	}
	creds := []credDef{
		{"MigrateMe Machine Credential", 1, orgCorpID, map[string]interface{}{
			"username": "ansible", "password": "ansible123", "become_password": "ansible123",
		}},
		{"MigrateMe SCM Credential", 2, orgCorpID, map[string]interface{}{
			"username": "git", "password": "gitpass123",
		}},
		{"MigrateMe Vault Credential", 3, orgCorpID, map[string]interface{}{
			"vault_password": "vaultpass123",
		}},
		{"Ops Machine Credential", 1, orgOpsID, map[string]interface{}{
			"username": "ops-ansible", "password": "ops-ansible123",
		}},
		{"MigrateMe API Token", ctID, orgCorpID, map[string]interface{}{
			"api_url": "https://api.example.com", "api_token": "tok-abc123",
		}},
	}
	credIDs := make(map[string]int)
	for _, cr := range creds {
		id, err := ensure("/api/controller/v2/credentials/", cr.name, map[string]interface{}{
			"name": cr.name, "credential_type": cr.credType,
			"organization": cr.orgID, "inputs": cr.inputs,
		})
		if err != nil {
			return fmt.Errorf("credential %s: %w", cr.name, err)
		}
		credIDs[cr.name] = id
		log(fmt.Sprintf("  Credential: %s (id=%d)", cr.name, id))
	}

	// 6. Projects
	log("\n=== Creating Projects ===")
	type projDef struct {
		name   string
		orgID  int
		scmURL string
		branch string
	}
	projects := []projDef{
		{"MigrateMe Sample Playbooks", orgCorpID, "https://github.com/ansible/ansible-tower-samples.git", "master"},
		{"Ops Automation Playbooks", orgOpsID, "https://github.com/ansible/ansible-examples.git", "master"},
	}
	projectIDs := make(map[string]int)
	for _, pr := range projects {
		id, err := ensure("/api/controller/v2/projects/", pr.name, map[string]interface{}{
			"name": pr.name, "organization": pr.orgID,
			"scm_type": "git", "scm_url": pr.scmURL, "scm_branch": pr.branch,
		})
		if err != nil {
			return fmt.Errorf("project %s: %w", pr.name, err)
		}
		projectIDs[pr.name] = id
		log(fmt.Sprintf("  Project: %s (id=%d)", pr.name, id))
	}

	// Wait for project sync
	log("  Waiting for project sync...")
	for name, id := range projectIDs {
		if err := p.waitForProject(id, 120*time.Second); err != nil {
			log(fmt.Sprintf("  WARNING: project %s sync: %v", name, err))
		} else {
			log(fmt.Sprintf("  Project %s synced successfully", name))
		}
	}

	// 7. Inventories, Hosts, Groups
	log("\n=== Creating Inventories ===")
	type hostDef struct {
		name string
		vars string
	}
	type groupDef struct {
		name  string
		hosts []string
	}
	type invDef struct {
		name   string
		orgID  int
		hosts  []hostDef
		groups []groupDef
	}
	inventories := []invDef{
		{"MigrateMe Dev Inventory", orgCorpID,
			[]hostDef{
				{"dev-web-01.example.com", `{"ansible_host": "192.168.1.10"}`},
				{"dev-web-02.example.com", `{"ansible_host": "192.168.1.11"}`},
				{"dev-db-01.example.com", `{"ansible_host": "192.168.1.20"}`},
			},
			[]groupDef{
				{"webservers", []string{"dev-web-01.example.com", "dev-web-02.example.com"}},
				{"databases", []string{"dev-db-01.example.com"}},
			},
		},
		{"MigrateMe Prod Inventory", orgCorpID,
			[]hostDef{
				{"prod-web-01.example.com", `{"ansible_host": "10.0.1.10"}`},
				{"prod-web-02.example.com", `{"ansible_host": "10.0.1.11"}`},
				{"prod-web-03.example.com", `{"ansible_host": "10.0.1.12"}`},
				{"prod-db-01.example.com", `{"ansible_host": "10.0.1.20"}`},
				{"prod-db-02.example.com", `{"ansible_host": "10.0.1.21"}`},
			},
			[]groupDef{
				{"webservers", []string{"prod-web-01.example.com", "prod-web-02.example.com", "prod-web-03.example.com"}},
				{"databases", []string{"prod-db-01.example.com", "prod-db-02.example.com"}},
			},
		},
		{"Ops Network Inventory", orgOpsID,
			[]hostDef{
				{"core-sw-01.ops.local", `{"ansible_host": "172.16.0.1", "ansible_network_os": "ios"}`},
				{"core-sw-02.ops.local", `{"ansible_host": "172.16.0.2", "ansible_network_os": "ios"}`},
				{"fw-01.ops.local", `{"ansible_host": "172.16.0.10", "ansible_network_os": "asa"}`},
				{"wan-rt-01.ops.local", `{"ansible_host": "172.16.0.20", "ansible_network_os": "iosxr"}`},
			},
			nil,
		},
	}

	invIDs := make(map[string]int)
	for _, inv := range inventories {
		invID, err := ensure("/api/controller/v2/inventories/", inv.name, map[string]interface{}{
			"name": inv.name, "organization": inv.orgID,
		})
		if err != nil {
			return fmt.Errorf("inventory %s: %w", inv.name, err)
		}
		invIDs[inv.name] = invID
		log(fmt.Sprintf("  Inventory: %s (id=%d)", inv.name, invID))

		// Create hosts
		hostIDs := make(map[string]int)
		for _, h := range inv.hosts {
			hID, err := ensure(
				fmt.Sprintf("/api/controller/v2/inventories/%d/hosts/", invID),
				h.name,
				map[string]interface{}{"name": h.name, "variables": h.vars},
			)
			if err != nil {
				log(fmt.Sprintf("    WARNING: host %s: %v", h.name, err))
				continue
			}
			hostIDs[h.name] = hID
			log(fmt.Sprintf("    Host: %s (id=%d)", h.name, hID))
		}

		// Create groups and associate hosts
		for _, g := range inv.groups {
			gID, err := ensure(
				fmt.Sprintf("/api/controller/v2/inventories/%d/groups/", invID),
				g.name,
				map[string]interface{}{"name": g.name},
			)
			if err != nil {
				log(fmt.Sprintf("    WARNING: group %s: %v", g.name, err))
				continue
			}
			log(fmt.Sprintf("    Group: %s (id=%d)", g.name, gID))
			for _, hName := range g.hosts {
				if hID, ok := hostIDs[hName]; ok {
					associate(fmt.Sprintf("/api/controller/v2/groups/%d/hosts/", gID), hID)
				}
			}
		}
	}

	// 8. Job Templates
	log("\n=== Creating Job Templates ===")
	type jtDef struct {
		name      string
		project   string
		inventory string
		playbook  string
		creds     []string
	}
	jts := []jtDef{
		{"MigrateMe - Hello World", "MigrateMe Sample Playbooks", "MigrateMe Dev Inventory", "hello_world.yml", []string{"MigrateMe Machine Credential"}},
		{"MigrateMe - Deploy App (Dev)", "MigrateMe Sample Playbooks", "MigrateMe Dev Inventory", "hello_world.yml", []string{"MigrateMe Machine Credential"}},
		{"MigrateMe - Deploy App (Prod)", "MigrateMe Sample Playbooks", "MigrateMe Prod Inventory", "hello_world.yml", []string{"MigrateMe Machine Credential"}},
		{"MigrateMe - DB Backup", "MigrateMe Sample Playbooks", "MigrateMe Dev Inventory", "hello_world.yml", []string{"MigrateMe Machine Credential", "MigrateMe Vault Credential"}},
		{"Ops - Network Audit", "Ops Automation Playbooks", "Ops Network Inventory", "hello_world.yml", []string{"Ops Machine Credential"}},
	}
	jtIDs := make(map[string]int)
	for _, jt := range jts {
		id, err := ensure("/api/controller/v2/job_templates/", jt.name, map[string]interface{}{
			"name": jt.name, "project": projectIDs[jt.project],
			"inventory": invIDs[jt.inventory], "playbook": jt.playbook,
		})
		if err != nil {
			log(fmt.Sprintf("  WARNING: job template %s: %v", jt.name, err))
			continue
		}
		jtIDs[jt.name] = id
		log(fmt.Sprintf("  Job Template: %s (id=%d)", jt.name, id))

		// Associate credentials
		for _, credName := range jt.creds {
			if cid, ok := credIDs[credName]; ok {
				associate(fmt.Sprintf("/api/controller/v2/job_templates/%d/credentials/", id), cid)
			}
		}
	}

	// 8b. Schedules
	log("\n=== Creating Schedules ===")
	type schedDef struct {
		name  string
		jtKey string
		rrule string
	}
	schedules := []schedDef{
		{"Daily Hello World", "MigrateMe - Hello World",
			"DTSTART:20250101T080000Z RRULE:FREQ=DAILY;INTERVAL=1"},
		{"Weekly DB Backup", "MigrateMe - DB Backup",
			"DTSTART:20250101T020000Z RRULE:FREQ=WEEKLY;INTERVAL=1;BYDAY=SU"},
	}
	for _, s := range schedules {
		jtID := jtIDs[s.jtKey]
		if jtID == 0 {
			log(fmt.Sprintf("  WARNING: schedule %s: JT %s not found", s.name, s.jtKey))
			continue
		}
		existing, err := c.FindByName("/api/controller/v2/schedules/", s.name)
		if err != nil {
			log(fmt.Sprintf("  WARNING: schedule %s: %v", s.name, err))
			continue
		}
		if existing != nil {
			log(fmt.Sprintf("  Schedule: %s (existing id=%d)", s.name, resourceID(existing)))
			continue
		}
		body, _, err := c.Post(fmt.Sprintf("/api/controller/v2/job_templates/%d/schedules/", jtID),
			map[string]interface{}{"name": s.name, "rrule": s.rrule})
		if err != nil {
			log(fmt.Sprintf("  WARNING: schedule %s: %v", s.name, err))
			continue
		}
		var created models.Resource
		json.Unmarshal(body, &created)
		log(fmt.Sprintf("  Schedule: %s (id=%d)", s.name, resourceID(created)))
	}

	// 8c. Surveys
	log("\n=== Creating Surveys ===")
	type surveyDef struct {
		jtKey string
		spec  map[string]interface{}
	}
	surveys := []surveyDef{
		{"MigrateMe - Deploy App (Dev)", map[string]interface{}{
			"name":        "Deploy Parameters",
			"description": "Deployment configuration",
			"spec": []map[string]interface{}{
				{
					"question_name": "Target Environment",
					"variable":      "target_env",
					"type":          "multiplechoice",
					"required":      true,
					"default":       "dev",
					"choices":       "dev\nstaging",
				},
				{
					"question_name": "App Version",
					"variable":      "app_version",
					"type":          "text",
					"required":      true,
					"default":       "latest",
				},
			},
		}},
	}
	for _, sv := range surveys {
		jtID := jtIDs[sv.jtKey]
		if jtID == 0 {
			log(fmt.Sprintf("  WARNING: survey for %s: JT not found", sv.jtKey))
			continue
		}
		_, _, err := c.Post(fmt.Sprintf("/api/controller/v2/job_templates/%d/survey_spec/", jtID), sv.spec)
		if err != nil {
			log(fmt.Sprintf("  WARNING: survey for %s: %v", sv.jtKey, err))
			continue
		}
		_, _, err = c.Patch(fmt.Sprintf("/api/controller/v2/job_templates/%d/", jtID),
			map[string]interface{}{"survey_enabled": true})
		if err != nil {
			log(fmt.Sprintf("  WARNING: enabling survey for %s: %v", sv.jtKey, err))
			continue
		}
		log(fmt.Sprintf("  Survey: %s (jt_id=%d)", sv.jtKey, jtID))
	}

	// 9. Workflow Job Template
	log("\n=== Creating Workflow Job Templates ===")
	wfjtID, err := ensure("/api/controller/v2/workflow_job_templates/", "MigrateMe - Full Deploy Pipeline", map[string]interface{}{
		"name": "MigrateMe - Full Deploy Pipeline", "organization": orgCorpID,
		"description": "Full deployment pipeline: backup → dev deploy → prod deploy",
	})
	if err != nil {
		return fmt.Errorf("workflow: %w", err)
	}
	log(fmt.Sprintf("  Workflow: MigrateMe - Full Deploy Pipeline (id=%d)", wfjtID))

	// Create workflow nodes (idempotent: reuse existing nodes by unified_job_template)
	type nodeDef struct {
		name string
		jtID int
	}
	nodes := []nodeDef{
		{"backup", jtIDs["MigrateMe - DB Backup"]},
		{"deploy_dev", jtIDs["MigrateMe - Deploy App (Dev)"]},
		{"deploy_prod", jtIDs["MigrateMe - Deploy App (Prod)"]},
	}

	// Fetch existing nodes to avoid duplicates
	existingNodes, _ := c.GetAll(fmt.Sprintf("/api/controller/v2/workflow_job_templates/%d/workflow_nodes/", wfjtID))
	existingByJT := make(map[int]int)
	for _, en := range existingNodes {
		if ujtID := intField(en, "unified_job_template"); ujtID > 0 {
			existingByJT[ujtID] = resourceID(en)
		}
	}

	nodeIDs := make([]int, len(nodes))
	for i, n := range nodes {
		if n.jtID == 0 {
			continue
		}
		if existingID, ok := existingByJT[n.jtID]; ok {
			nodeIDs[i] = existingID
			log(fmt.Sprintf("    Node: %s (existing node_id=%d, jt_id=%d)", n.name, existingID, n.jtID))
			continue
		}
		body, _, err := c.Post(fmt.Sprintf("/api/controller/v2/workflow_job_templates/%d/workflow_nodes/", wfjtID),
			map[string]interface{}{"unified_job_template": n.jtID})
		if err != nil {
			log(fmt.Sprintf("  WARNING: workflow node %s: %v", n.name, err))
			continue
		}
		var created models.Resource
		json.Unmarshal(body, &created)
		nodeIDs[i] = resourceID(created)
		log(fmt.Sprintf("    Node: %s (node_id=%d, jt_id=%d)", n.name, nodeIDs[i], n.jtID))
	}

	// Wire edges: backup --success--> deploy_dev --success--> deploy_prod
	if nodeIDs[0] > 0 && nodeIDs[1] > 0 {
		c.Post(fmt.Sprintf("/api/controller/v2/workflow_job_template_nodes/%d/success_nodes/", nodeIDs[0]),
			map[string]interface{}{"id": nodeIDs[1]})
	}
	if nodeIDs[1] > 0 && nodeIDs[2] > 0 {
		c.Post(fmt.Sprintf("/api/controller/v2/workflow_job_template_nodes/%d/success_nodes/", nodeIDs[1]),
			map[string]interface{}{"id": nodeIDs[2]})
	}

	// 10. RBAC Roles
	log("\n=== Assigning Team Roles ===")
	type roleDef struct {
		teamName   string
		objectType string
		objectName string
		objectID   int
		roleField  string
	}
	roleAssignments := []roleDef{
		{"DevOps", "organizations", "MigrateMe-Corp", orgCorpID, "admin_role"},
		{"DBA", "organizations", "MigrateMe-Corp", orgCorpID, "read_role"},
		{"Security", "organizations", "MigrateMe-Corp", orgCorpID, "read_role"},
		{"App Development", "organizations", "MigrateMe-Corp", orgCorpID, "member_role"},
		{"Network Operations", "organizations", "MigrateMe-Ops", orgOpsID, "admin_role"},
		{"Infrastructure", "organizations", "MigrateMe-Ops", orgOpsID, "member_role"},
		{"DevOps", "job_templates", "MigrateMe - Deploy App (Dev)", jtIDs["MigrateMe - Deploy App (Dev)"], "execute_role"},
		{"DevOps", "job_templates", "MigrateMe - Deploy App (Prod)", jtIDs["MigrateMe - Deploy App (Prod)"], "execute_role"},
		{"DBA", "job_templates", "MigrateMe - DB Backup", jtIDs["MigrateMe - DB Backup"], "execute_role"},
		{"Security", "inventories", "MigrateMe Prod Inventory", invIDs["MigrateMe Prod Inventory"], "read_role"},
		{"App Development", "job_templates", "MigrateMe - Hello World", jtIDs["MigrateMe - Hello World"], "execute_role"},
		{"Network Operations", "job_templates", "Ops - Network Audit", jtIDs["Ops - Network Audit"], "execute_role"},
		{"Network Operations", "inventories", "Ops Network Inventory", invIDs["Ops Network Inventory"], "admin_role"},
		{"DevOps", "credentials", "MigrateMe Machine Credential", credIDs["MigrateMe Machine Credential"], "use_role"},
		{"DBA", "credentials", "MigrateMe Vault Credential", credIDs["MigrateMe Vault Credential"], "use_role"},
		{"DevOps", "projects", "MigrateMe Sample Playbooks", projectIDs["MigrateMe Sample Playbooks"], "use_role"},
		{"Network Operations", "projects", "Ops Automation Playbooks", projectIDs["Ops Automation Playbooks"], "use_role"},
		{"DevOps", "workflow_job_templates", "MigrateMe - Full Deploy Pipeline", wfjtID, "execute_role"},
		{"Infrastructure", "inventories", "Ops Network Inventory", invIDs["Ops Network Inventory"], "use_role"},
	}

	for _, ra := range roleAssignments {
		if ra.objectID == 0 {
			continue
		}
		var obj map[string]interface{}
		objPath := fmt.Sprintf("/api/controller/v2/%s/%d/", ra.objectType, ra.objectID)
		if err := c.GetJSON(objPath, nil, &obj); err != nil {
			log(fmt.Sprintf("  WARNING: %s role %s on %s: %v", ra.teamName, ra.roleField, ra.objectName, err))
			continue
		}
		roleID := extractRoleID(obj, ra.roleField)
		if roleID == 0 {
			log(fmt.Sprintf("  WARNING: role %s not found on %s", ra.roleField, ra.objectName))
			continue
		}
		c.Post(fmt.Sprintf("/api/controller/v2/roles/%d/teams/", roleID), map[string]interface{}{"id": teamIDs[ra.teamName]})
		log(fmt.Sprintf("  %s → %s.%s", ra.teamName, ra.objectName, ra.roleField))
	}

	log("\nPopulate complete!")
	return nil
}

// Cleanup deletes non-default objects from AAP in reverse dependency order.
func (p *AAPPlatform) Cleanup(logger func(string)) error {
	log := logger

	// Deletion order (reverse dependency)
	deleteOrder := []models.ResourceType{
		findResource(aapResources, "schedules"),
		findResource(aapResources, "workflow_job_templates"),
		findResource(aapResources, "job_templates"),
		findResource(aapResources, "inventories"),
		findResource(aapResources, "projects"),
		findResource(aapResources, "credentials"),
		findResource(aapResources, "credential_types"),
		findResource(aapResources, "users"),
		findResource(aapResources, "teams"),
		findResource(aapResources, "organizations"),
	}

	deleted, skipped, failed := 0, 0, 0

	for _, rt := range deleteOrder {
		log(fmt.Sprintf("\n--- Cleaning %s ---", rt.Label))

		resources, err := p.client.GetAll(rt.APIPath)
		if err != nil {
			log(fmt.Sprintf("  ERROR listing %s: %v", rt.Label, err))
			failed++
			continue
		}

		for _, res := range resources {
			name := resourceName(res)
			id := resourceID(res)

			// Skip managed objects
			if managed, ok := res["managed"].(bool); ok && managed {
				log(fmt.Sprintf("  SKIP %s (managed)", name))
				skipped++
				continue
			}

			// Skip known defaults
			if rt.Skip != nil && rt.Skip[name] {
				log(fmt.Sprintf("  SKIP %s (default)", name))
				skipped++
				continue
			}

			// Skip managed credential types (kind != "cloud" and kind != "net")
			if rt.Name == "credential_types" {
				if managed, ok := res["managed"].(bool); ok && managed {
					skipped++
					continue
				}
			}

			err := p.client.Delete(fmt.Sprintf("%s%d/", rt.APIPath, id))
			if err != nil {
				log(fmt.Sprintf("  FAIL %s (id=%d): %v", name, id, err))
				failed++
			} else {
				log(fmt.Sprintf("  DELETED %s (id=%d)", name, id))
				deleted++
			}
		}
	}

	log(fmt.Sprintf("\nCleanup complete: %d deleted, %d skipped, %d failed", deleted, skipped, failed))
	return nil
}

// Export downloads AAP assets in breadth-first dependency order.
func (p *AAPPlatform) Export(outputDir string, logger func(string)) error {
	log := logger

	downloaded := map[string]map[int]bool{
		"workflow_job_templates":  {},
		"job_templates":           {},
		"projects":                {},
		"inventories":             {},
		"credentials":             {},
		"execution_environments":  {},
		"organizations":           {},
	}

	fileCount := 0

	writeJSON := func(dir, filename string, data interface{}) error {
		dirPath := filepath.Join(outputDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return err
		}
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		fileCount++
		return os.WriteFile(filepath.Join(dirPath, filename), b, 0644)
	}

	safeName := func(name string) string {
		r := strings.NewReplacer(" ", "_", "/", "_", "\\", "_")
		return r.Replace(name)
	}

	// Fetch helper (single object by ID)
	fetchOne := func(path string, id int) (map[string]interface{}, error) {
		var obj map[string]interface{}
		err := p.client.GetJSON(fmt.Sprintf("%s%d/", path, id), nil, &obj)
		return obj, err
	}

	// Helper to download a dependency
	downloadOrg := func(id int) {
		if id == 0 || downloaded["organizations"][id] {
			return
		}
		downloaded["organizations"][id] = true
		obj, err := fetchOne("/api/controller/v2/organizations/", id)
		if err != nil {
			log(fmt.Sprintf("  WARNING: org %d: %v", id, err))
			return
		}
		name := obj["name"].(string)
		writeJSON("organizations", fmt.Sprintf("%d_%s.json", id, safeName(name)), obj)
		log(fmt.Sprintf("  Organization: %s (id=%d)", name, id))
	}

	downloadCred := func(id int) {
		if id == 0 || downloaded["credentials"][id] {
			return
		}
		downloaded["credentials"][id] = true
		obj, err := fetchOne("/api/controller/v2/credentials/", id)
		if err != nil {
			log(fmt.Sprintf("  WARNING: credential %d: %v", id, err))
			return
		}
		name := obj["name"].(string)
		obj["inputs"] = map[string]interface{}{"_note": "Sensitive data removed"}
		writeJSON("credentials", fmt.Sprintf("%d_%s.json", id, safeName(name)), obj)
		log(fmt.Sprintf("  Credential: %s (id=%d)", name, id))
		if orgID := intField(obj, "organization"); orgID > 0 {
			downloadOrg(orgID)
		}
	}

	downloadEE := func(id int) {
		if id == 0 || downloaded["execution_environments"][id] {
			return
		}
		downloaded["execution_environments"][id] = true
		obj, err := fetchOne("/api/controller/v2/execution_environments/", id)
		if err != nil {
			log(fmt.Sprintf("  WARNING: EE %d: %v", id, err))
			return
		}
		name := obj["name"].(string)
		writeJSON("execution_environments", fmt.Sprintf("%d_%s.json", id, safeName(name)), obj)
		log(fmt.Sprintf("  Execution Environment: %s (id=%d)", name, id))
		if orgID := intField(obj, "organization"); orgID > 0 {
			downloadOrg(orgID)
		}
	}

	downloadProject := func(id int) {
		if id == 0 || downloaded["projects"][id] {
			return
		}
		downloaded["projects"][id] = true
		obj, err := fetchOne("/api/controller/v2/projects/", id)
		if err != nil {
			log(fmt.Sprintf("  WARNING: project %d: %v", id, err))
			return
		}
		name := obj["name"].(string)
		writeJSON("projects", fmt.Sprintf("%d_%s.json", id, safeName(name)), obj)
		log(fmt.Sprintf("  Project: %s (id=%d)", name, id))
		if orgID := intField(obj, "organization"); orgID > 0 {
			downloadOrg(orgID)
		}
		// SCM credential
		if sf, ok := obj["summary_fields"].(map[string]interface{}); ok {
			if cred, ok := sf["credential"].(map[string]interface{}); ok {
				if credID := intField(cred, "id"); credID > 0 {
					downloadCred(credID)
				}
			}
		}
	}

	downloadInventory := func(id int) {
		if id == 0 || downloaded["inventories"][id] {
			return
		}
		downloaded["inventories"][id] = true
		obj, err := fetchOne("/api/controller/v2/inventories/", id)
		if err != nil {
			log(fmt.Sprintf("  WARNING: inventory %d: %v", id, err))
			return
		}
		name := obj["name"].(string)
		writeJSON("inventories", fmt.Sprintf("%d_%s.json", id, safeName(name)), obj)
		log(fmt.Sprintf("  Inventory: %s (id=%d)", name, id))
		if orgID := intField(obj, "organization"); orgID > 0 {
			downloadOrg(orgID)
		}
		// Inventory sources
		sources, err := p.client.GetAll(fmt.Sprintf("/api/controller/v2/inventories/%d/inventory_sources/", id))
		if err == nil && len(sources) > 0 {
			writeJSON("inventories", fmt.Sprintf("%d_%s_sources.json", id, safeName(name)), sources)
		}
	}

	downloadJT := func(id int) {
		if id == 0 || downloaded["job_templates"][id] {
			return
		}
		downloaded["job_templates"][id] = true
		obj, err := fetchOne("/api/controller/v2/job_templates/", id)
		if err != nil {
			log(fmt.Sprintf("  WARNING: JT %d: %v", id, err))
			return
		}
		name := obj["name"].(string)
		writeJSON("job_templates", fmt.Sprintf("%d_%s_details.json", id, safeName(name)), obj)
		log(fmt.Sprintf("  Job Template: %s (id=%d)", name, id))

		// Survey (optional)
		var survey map[string]interface{}
		if err := p.client.GetJSON(fmt.Sprintf("/api/controller/v2/job_templates/%d/survey_spec/", id), nil, &survey); err == nil {
			writeJSON("job_templates", fmt.Sprintf("%d_%s_survey.json", id, safeName(name)), survey)
		}

		// Dependencies
		if projID := intField(obj, "project"); projID > 0 {
			downloadProject(projID)
		}
		if invID := intField(obj, "inventory"); invID > 0 {
			downloadInventory(invID)
		}
		if eeID := intField(obj, "execution_environment"); eeID > 0 {
			downloadEE(eeID)
		}
		// Credentials from summary_fields
		if sf, ok := obj["summary_fields"].(map[string]interface{}); ok {
			if creds, ok := sf["credentials"].([]interface{}); ok {
				for _, c := range creds {
					if cm, ok := c.(map[string]interface{}); ok {
						if credID := intField(cm, "id"); credID > 0 {
							downloadCred(credID)
						}
					}
				}
			}
		}
	}

	// Start: Fetch all workflow job templates
	log("=== Downloading Workflow Job Templates ===")
	workflows, err := p.client.GetAll("/api/controller/v2/workflow_job_templates/")
	if err != nil {
		return fmt.Errorf("fetching workflows: %w", err)
	}

	writeJSON("workflow_job_templates", "_all_workflows.json", workflows)

	for _, wf := range workflows {
		wfID := resourceID(wf)
		name := resourceName(wf)
		if wfID == 0 {
			continue
		}
		downloaded["workflow_job_templates"][wfID] = true
		log(fmt.Sprintf("\nWorkflow: %s (id=%d)", name, wfID))

		// Details
		details, err := fetchOne("/api/controller/v2/workflow_job_templates/", wfID)
		if err != nil {
			log(fmt.Sprintf("  WARNING: workflow details %d: %v", wfID, err))
			continue
		}
		writeJSON("workflow_job_templates", fmt.Sprintf("%d_%s_details.json", wfID, safeName(name)), details)

		// Nodes
		nodes, err := p.client.GetAll(fmt.Sprintf("/api/controller/v2/workflow_job_templates/%d/workflow_nodes/", wfID))
		if err != nil {
			log(fmt.Sprintf("  WARNING: workflow nodes %d: %v", wfID, err))
			continue
		}
		writeJSON("workflow_job_templates", fmt.Sprintf("%d_%s_nodes.json", wfID, safeName(name)), nodes)

		// Survey (optional)
		var survey map[string]interface{}
		if err := p.client.GetJSON(fmt.Sprintf("/api/controller/v2/workflow_job_templates/%d/survey_spec/", wfID), nil, &survey); err == nil {
			writeJSON("workflow_job_templates", fmt.Sprintf("%d_%s_survey.json", wfID, safeName(name)), survey)
		}

		// Process nodes to find job template dependencies
		for _, node := range nodes {
			// Extract unified_job_template ID from the node
			if ujt := intField(node, "unified_job_template"); ujt > 0 {
				// Check if it's a job template by looking at related URLs or summary_fields
				if sf, ok := node["summary_fields"].(map[string]interface{}); ok {
					if ujtData, ok := sf["unified_job_template"].(map[string]interface{}); ok {
						if ujtType, _ := ujtData["unified_job_type"].(string); ujtType == "job" {
							downloadJT(ujt)
						}
					}
				}
			}
		}
	}

	log(fmt.Sprintf("\n=== Export complete: %d JSON files created ===", fileCount))
	counts := make(map[string]int)
	for k, v := range downloaded {
		counts[k] = len(v)
	}
	for k, v := range counts {
		if v > 0 {
			log(fmt.Sprintf("  %s: %d", k, v))
		}
	}

	return nil
}

// waitForProject polls a project until its status is "successful" or "failed".
func (p *AAPPlatform) waitForProject(id int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var proj map[string]interface{}
		err := p.client.GetJSON(fmt.Sprintf("/api/controller/v2/projects/%d/", id), nil, &proj)
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

func findResource(resources []models.ResourceType, name string) models.ResourceType {
	for _, r := range resources {
		if r.Name == name {
			return r
		}
	}
	return models.ResourceType{}
}

func resourceName(r models.Resource) string {
	if r == nil {
		return ""
	}
	if name, ok := r["name"].(string); ok {
		return name
	}
	if name, ok := r["username"].(string); ok {
		return name
	}
	return ""
}

func intField(obj map[string]interface{}, field string) int {
	if obj == nil {
		return 0
	}
	switch v := obj[field].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	}
	return 0
}
