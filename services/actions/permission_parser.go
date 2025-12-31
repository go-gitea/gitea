// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"

	"github.com/nektos/act/pkg/jobparser"
	"gopkg.in/yaml.v3"
)

// ParseWorkflowPermissions extracts workflow-level permissions from a SingleWorkflow
// Returns the default permissions based on repository settings if no workflow permissions are specified
func ParseWorkflowPermissions(wf *jobparser.SingleWorkflow, defaultPerms repo_model.ActionsTokenPermissions) repo_model.ActionsTokenPermissions {
	if wf == nil {
		return defaultPerms
	}

	// Check if workflow has RawPermissions
	rawPerms := wf.RawPermissions
	if rawPerms.Kind == yaml.ScalarNode && rawPerms.Value == "" {
		return defaultPerms
	}

	return parseRawPermissions(&rawPerms, defaultPerms)
}

// ParseJobPermissions extracts job-level permissions, falling back to workflow defaults
func ParseJobPermissions(job *jobparser.Job, workflowPerms repo_model.ActionsTokenPermissions) repo_model.ActionsTokenPermissions {
	if job == nil {
		return workflowPerms
	}

	// Check if job has RawPermissions
	rawPerms := job.RawPermissions
	if rawPerms.Kind == yaml.ScalarNode && rawPerms.Value == "" {
		return workflowPerms
	}

	return parseRawPermissions(&rawPerms, workflowPerms)
}

// parseRawPermissions parses a YAML permissions node into ActionsTokenPermissions
func parseRawPermissions(rawPerms *yaml.Node, defaultPerms repo_model.ActionsTokenPermissions) repo_model.ActionsTokenPermissions {
	if rawPerms == nil || (rawPerms.Kind == yaml.ScalarNode && rawPerms.Value == "") {
		return defaultPerms
	}

	// Handle scalar values: "read-all" or "write-all"
	if rawPerms.Kind == yaml.ScalarNode {
		switch rawPerms.Value {
		case "read-all":
			return repo_model.ActionsTokenPermissions{
				Contents:     perm.AccessModeRead,
				Issues:       perm.AccessModeRead,
				PullRequests: perm.AccessModeRead,
				Packages:     perm.AccessModeRead,
				Actions:      perm.AccessModeRead,
				Wiki:         perm.AccessModeRead,
			}
		case "write-all":
			return repo_model.ActionsTokenPermissions{
				Contents:     perm.AccessModeWrite,
				Issues:       perm.AccessModeWrite,
				PullRequests: perm.AccessModeWrite,
				Packages:     perm.AccessModeWrite,
				Actions:      perm.AccessModeWrite,
				Wiki:         perm.AccessModeWrite,
			}
		}
		return defaultPerms
	}

	// Handle mapping: individual permission scopes
	if rawPerms.Kind == yaml.MappingNode {
		result := defaultPerms // Start with defaults

		for i := 0; i < len(rawPerms.Content); i += 2 {
			if i+1 >= len(rawPerms.Content) {
				break
			}
			keyNode := rawPerms.Content[i]
			valueNode := rawPerms.Content[i+1]

			if keyNode.Kind != yaml.ScalarNode || valueNode.Kind != yaml.ScalarNode {
				continue
			}

			scope := keyNode.Value
			accessStr := valueNode.Value
			accessMode := parseAccessMode(accessStr)

			// Map GitHub Actions scopes to Gitea units
			switch scope {
			case "contents":
				result.Contents = accessMode
			case "issues":
				result.Issues = accessMode
			case "pull-requests":
				result.PullRequests = accessMode
			case "packages":
				result.Packages = accessMode
			case "actions":
				result.Actions = accessMode
			case "wiki":
				result.Wiki = accessMode
				// Additional GitHub scopes we don't explicitly handle yet:
				// These fall through to defaults
				// - deployments, environments, id-token, pages, repository-projects, security-events, statuses
			}
		}

		return result
	}

	return defaultPerms
}

// parseAccessMode converts a string access level to perm.AccessMode
func parseAccessMode(s string) perm.AccessMode {
	switch s {
	case "write":
		return perm.AccessModeWrite
	case "read":
		return perm.AccessModeRead
	case "none":
		return perm.AccessModeNone
	default:
		return perm.AccessModeNone
	}
}
