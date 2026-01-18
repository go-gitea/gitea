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

	// Unwrap DocumentNode if present (yaml.Unmarshal wraps content in DocumentNode)
	node := rawPerms
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}

	// Check for empty node after unwrapping
	if node == nil || (node.Kind == yaml.ScalarNode && node.Value == "") {
		return defaultPerms
	}

	// Handle scalar values: "read-all" or "write-all"
	if node.Kind == yaml.ScalarNode {
		switch node.Value {
		case "read-all":
			return repo_model.ActionsTokenPermissions{
				Code:         perm.AccessModeRead,
				Issues:       perm.AccessModeRead,
				PullRequests: perm.AccessModeRead,
				Packages:     perm.AccessModeRead,
				Actions:      perm.AccessModeRead,
				Wiki:         perm.AccessModeRead,
				Releases:     perm.AccessModeRead,
				Projects:     perm.AccessModeRead,
			}
		case "write-all":
			return repo_model.ActionsTokenPermissions{
				Code:         perm.AccessModeWrite,
				Issues:       perm.AccessModeWrite,
				PullRequests: perm.AccessModeWrite,
				Packages:     perm.AccessModeWrite,
				Actions:      perm.AccessModeWrite,
				Wiki:         perm.AccessModeWrite,
				Releases:     perm.AccessModeWrite,
				Projects:     perm.AccessModeWrite,
			}
		}
		return defaultPerms
	}

	// Handle mapping: individual permission scopes
	if node.Kind == yaml.MappingNode {
		result := defaultPerms // Start with defaults

		// Collect all scopes into a map first to handle priority
		scopes := make(map[string]perm.AccessMode)
		for i := 0; i < len(node.Content); i += 2 {
			if i+1 >= len(node.Content) {
				break
			}
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]

			if keyNode.Kind != yaml.ScalarNode || valueNode.Kind != yaml.ScalarNode {
				continue
			}

			scopes[keyNode.Value] = parseAccessMode(valueNode.Value)
		}

		// 1. Apply 'contents' first (lower priority)
		if mode, ok := scopes["contents"]; ok {
			result.Code = mode
			result.Releases = mode
		}

		// 2. Apply all other scopes (overwrites contents if specified)
		for scope, mode := range scopes {
			switch scope {
			case "contents":
				// already handled
			case "code":
				result.Code = mode
			case "issues":
				result.Issues = mode
			case "pull-requests":
				result.PullRequests = mode
			case "packages":
				result.Packages = mode
			case "actions":
				result.Actions = mode
			case "wiki":
				result.Wiki = mode
			case "releases":
				result.Releases = mode
			case "projects":
				result.Projects = mode
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
