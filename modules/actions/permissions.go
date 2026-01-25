// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"strings"

	auth_model "code.gitea.io/gitea/models/auth"
	"gopkg.in/yaml.v3"
)

// WorkflowPermissions represents the permissions field in a workflow or job
type WorkflowPermissions struct {
	Actions       string `yaml:"actions"`
	Checks        string `yaml:"checks"`
	Contents      string `yaml:"contents"`
	Deployments   string `yaml:"deployments"`
	Issues        string `yaml:"issues"`
	Metadata      string `yaml:"metadata"`
	Packages      string `yaml:"packages"`
	PullRequests  string `yaml:"pull-requests"`
	Discussions   string `yaml:"discussions"`
	SecurityEvents string `yaml:"security-events"`
	Statuses      string `yaml:"statuses"`
}

// ParseWorkflowPermissions parses permissions from workflow YAML content
// It supports both workflow-level and job-level permissions
func ParseWorkflowPermissions(workflowContent []byte, jobID string) (*WorkflowPermissions, error) {
	var workflow struct {
		Permissions interface{} `yaml:"permissions"`
		Jobs        map[string]struct {
			Permissions interface{} `yaml:"permissions"`
		} `yaml:"jobs"`
	}

	if err := yaml.Unmarshal(workflowContent, &workflow); err != nil {
		return nil, err
	}

	// Check job-level permissions first (they override workflow-level)
	if jobID != "" && workflow.Jobs != nil {
		if job, ok := workflow.Jobs[jobID]; ok && job.Permissions != nil {
			return parsePermissionsField(job.Permissions)
		}
	}

	// Fall back to workflow-level permissions
	if workflow.Permissions != nil {
		return parsePermissionsField(workflow.Permissions)
	}

	// No permissions specified, return nil
	return nil, nil
}

// parsePermissionsField parses the permissions field which can be either:
// - A string: "read-all", "write-all", or "{}"
// - A map: detailed permissions for each scope
func parsePermissionsField(perms interface{}) (*WorkflowPermissions, error) {
	switch v := perms.(type) {
	case string:
		// Handle string format: "read-all", "write-all", or "{}"
		return parsePermissionsString(v), nil
	case map[string]interface{}:
		// Handle map format
		return parsePermissionsMap(v), nil
	default:
		return nil, nil
	}
}

func parsePermissionsString(perms string) *WorkflowPermissions {
	perms = strings.TrimSpace(perms)
	wp := &WorkflowPermissions{}

	switch perms {
	case "read-all":
		wp.Actions = "read"
		wp.Checks = "read"
		wp.Contents = "read"
		wp.Deployments = "read"
		wp.Issues = "read"
		wp.Metadata = "read"
		wp.Packages = "read"
		wp.PullRequests = "read"
		wp.Discussions = "read"
		wp.SecurityEvents = "read"
		wp.Statuses = "read"
	case "write-all":
		wp.Actions = "write"
		wp.Checks = "write"
		wp.Contents = "write"
		wp.Deployments = "write"
		wp.Issues = "write"
		wp.Metadata = "read" // metadata is always read-only
		wp.Packages = "write"
		wp.PullRequests = "write"
		wp.Discussions = "write"
		wp.SecurityEvents = "write"
		wp.Statuses = "write"
	case "{}":
		// Empty permissions - all set to none
		wp.Metadata = "read" // metadata is always at least read
	}

	return wp
}

func parsePermissionsMap(perms map[string]interface{}) *WorkflowPermissions {
	wp := &WorkflowPermissions{}

	if v, ok := perms["actions"].(string); ok {
		wp.Actions = v
	}
	if v, ok := perms["checks"].(string); ok {
		wp.Checks = v
	}
	if v, ok := perms["contents"].(string); ok {
		wp.Contents = v
	}
	if v, ok := perms["deployments"].(string); ok {
		wp.Deployments = v
	}
	if v, ok := perms["issues"].(string); ok {
		wp.Issues = v
	}
	if v, ok := perms["metadata"].(string); ok {
		wp.Metadata = v
	}
	if v, ok := perms["packages"].(string); ok {
		wp.Packages = v
	}
	if v, ok := perms["pull-requests"].(string); ok {
		wp.PullRequests = v
	}
	if v, ok := perms["discussions"].(string); ok {
		wp.Discussions = v
	}
	if v, ok := perms["security-events"].(string); ok {
		wp.SecurityEvents = v
	}
	if v, ok := perms["statuses"].(string); ok {
		wp.Statuses = v
	}

	// Ensure metadata is at least read
	if wp.Metadata == "" {
		wp.Metadata = "read"
	}

	return wp
}

// ToAccessTokenScopes converts workflow permissions to AccessTokenScope
func (wp *WorkflowPermissions) ToAccessTokenScopes() auth_model.AccessTokenScope {
	if wp == nil {
		return ""
	}

	scopes := make([]string, 0)

	// Helper to add scope based on permission level
	addScope := func(permission string, category auth_model.AccessTokenScopeCategory) {
		switch permission {
		case "read":
			scopes = append(scopes, string(auth_model.GetRequiredScopes(auth_model.Read, category)[0]))
		case "write":
			scopes = append(scopes, string(auth_model.GetRequiredScopes(auth_model.Write, category)[0]))
		case "none", "":
			// Don't add any scope
		}
	}

	// Map workflow permissions to token scopes
	addScope(wp.Contents, auth_model.AccessTokenScopeCategoryRepository)
	addScope(wp.Issues, auth_model.AccessTokenScopeCategoryIssue)
	addScope(wp.PullRequests, auth_model.AccessTokenScopeCategoryIssue)
	addScope(wp.Packages, auth_model.AccessTokenScopeCategoryPackage)

	// Metadata is always at least read
	if wp.Metadata == "read" || wp.Metadata == "" {
		scopes = append(scopes, string(auth_model.AccessTokenScopeReadRepository))
	}

	// Join all scopes
	if len(scopes) == 0 {
		return ""
	}

	scopeStr := ""
	for i, scope := range scopes {
		if i > 0 {
			scopeStr += ","
		}
		scopeStr += scope
	}

	return auth_model.AccessTokenScope(scopeStr)
}

// MergeWithDefaults merges workflow permissions with default repository permissions
// Workflow permissions take precedence over defaults
func (wp *WorkflowPermissions) MergeWithDefaults(defaultPerms *WorkflowPermissions) *WorkflowPermissions {
	if wp == nil {
		return defaultPerms
	}
	if defaultPerms == nil {
		return wp
	}

	merged := &WorkflowPermissions{}

	// Helper to merge individual permission
	merge := func(workflow, def string) string {
		if workflow != "" {
			return workflow
		}
		return def
	}

	merged.Actions = merge(wp.Actions, defaultPerms.Actions)
	merged.Checks = merge(wp.Checks, defaultPerms.Checks)
	merged.Contents = merge(wp.Contents, defaultPerms.Contents)
	merged.Deployments = merge(wp.Deployments, defaultPerms.Deployments)
	merged.Issues = merge(wp.Issues, defaultPerms.Issues)
	merged.Metadata = merge(wp.Metadata, defaultPerms.Metadata)
	merged.Packages = merge(wp.Packages, defaultPerms.Packages)
	merged.PullRequests = merge(wp.PullRequests, defaultPerms.PullRequests)
	merged.Discussions = merge(wp.Discussions, defaultPerms.Discussions)
	merged.SecurityEvents = merge(wp.SecurityEvents, defaultPerms.SecurityEvents)
	merged.Statuses = merge(wp.Statuses, defaultPerms.Statuses)

	// Ensure metadata is at least read
	if merged.Metadata == "" {
		merged.Metadata = "read"
	}

	return merged
}
