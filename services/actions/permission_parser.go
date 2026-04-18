// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/actions/jobparser"
	"code.gitea.io/gitea/modules/setting"

	"go.yaml.in/yaml/v4"
)

// ExtractJobPermissionsFromWorkflow extracts permissions from an already parsed workflow/job.
// It returns nil if neither workflow nor job explicitly specifies permissions.
func ExtractJobPermissionsFromWorkflow(flow *jobparser.SingleWorkflow, job *jobparser.Job) *repo_model.ActionsTokenPermissions {
	if flow == nil || job == nil {
		return nil
	}

	jobPerms := parseRawPermissionsExplicit(&job.RawPermissions)
	if jobPerms != nil {
		return jobPerms
	}

	workflowPerms := parseRawPermissionsExplicit(&flow.RawPermissions)
	if workflowPerms != nil {
		return workflowPerms
	}

	return nil
}

// parseRawPermissionsExplicit parses a YAML permissions node and returns only explicit scopes.
// It returns nil if the node does not explicitly specify permissions.
func parseRawPermissionsExplicit(rawPerms *yaml.Node) *repo_model.ActionsTokenPermissions {
	if rawPerms == nil || (rawPerms.Kind == yaml.ScalarNode && rawPerms.Value == "") {
		return nil
	}

	// Unwrap DocumentNode and resolve AliasNode
	node := rawPerms
	for node.Kind == yaml.DocumentNode || node.Kind == yaml.AliasNode {
		if node.Kind == yaml.DocumentNode {
			if len(node.Content) == 0 {
				return nil
			}
			node = node.Content[0]
		} else {
			node = node.Alias
		}
	}

	if node.Kind == yaml.ScalarNode && node.Value == "" {
		return nil
	}

	// Handle scalar values: "read-all" or "write-all"
	if node.Kind == yaml.ScalarNode {
		switch node.Value {
		case "read-all":
			return new(repo_model.MakeActionsTokenPermissions(perm.AccessModeRead))
		case "write-all":
			return new(repo_model.MakeActionsTokenPermissions(perm.AccessModeWrite))
		default:
			// Explicit but unrecognized scalar: return all-none permissions.
			return new(repo_model.MakeActionsTokenPermissions(perm.AccessModeNone))
		}
	}

	// Handle mapping: individual permission scopes
	if node.Kind == yaml.MappingNode {
		result := repo_model.MakeActionsTokenPermissions(perm.AccessModeNone)

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
			result.UnitAccessModes[unit.TypeCode] = mode
			result.UnitAccessModes[unit.TypeReleases] = mode
		}

		// 2. Apply all other scopes (overwrites contents if specified)
		for scope, mode := range scopes {
			switch scope {
			case "contents":
				// already handled
			case "code":
				result.UnitAccessModes[unit.TypeCode] = mode
			case "issues":
				result.UnitAccessModes[unit.TypeIssues] = mode
			case "pull-requests":
				result.UnitAccessModes[unit.TypePullRequests] = mode
			case "packages":
				result.UnitAccessModes[unit.TypePackages] = mode
			case "actions":
				result.UnitAccessModes[unit.TypeActions] = mode
			case "wiki":
				result.UnitAccessModes[unit.TypeWiki] = mode
			case "releases":
				result.UnitAccessModes[unit.TypeReleases] = mode
			case "projects":
				result.UnitAccessModes[unit.TypeProjects] = mode
			// Scopes github supports but gitea does not, see url for details
			// https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax
			case "artifact-metadata", "attestations", "checks", "deployments",
				"id-token", "models", "discussions", "pages", "security-events", "statuses":
				// not supported
			default:
				setting.PanicInDevOrTesting("Unrecognized permission scope: %s", scope)
			}
		}

		return &result
	}

	return nil
}

// parseAccessMode converts a string access level to perm.AccessMode
func parseAccessMode(s string) perm.AccessMode {
	switch s {
	case "write":
		return perm.AccessModeWrite
	case "read":
		return perm.AccessModeRead
	default:
		return perm.AccessModeNone
	}
}
