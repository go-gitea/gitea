// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	repo_model "code.gitea.io/gitea/models/repo"

	"github.com/nektos/act/pkg/jobparser"
	"gopkg.in/yaml.v3"
)

// ParsePermissionsFromYAMLNode parses the `permissions` yaml.Node into TokenPermissions.
// The permissions key can be:
//   - A scalar string: "read-all" or "write-all" (applies to all known scopes)
//   - A mapping: scope: level (e.g., contents: read, issues: write)
//   - Empty/zero: returns nil (meaning use defaults)
func ParsePermissionsFromYAMLNode(node *yaml.Node) (actions_model.TokenPermissions, error) {
	if node == nil || node.Kind == 0 {
		return nil, nil //nolint:nilnil
	}

	switch node.Kind {
	case yaml.ScalarNode:
		return parseScalarPermissions(node.Value)
	case yaml.MappingNode:
		return parseMappingPermissions(node)
	default:
		return nil, fmt.Errorf("unexpected permissions YAML node kind: %d", node.Kind)
	}
}

// allKnownScopes returns all the GitHub-compatible scope names we support
func allKnownScopes() []string {
	return []string{"contents", "issues", "pull-requests", "packages", "actions"}
}

// parseScalarPermissions handles "read-all" and "write-all"
func parseScalarPermissions(value string) (actions_model.TokenPermissions, error) {
	var level actions_model.TokenPermissionLevel
	switch value {
	case "read-all":
		level = actions_model.TokenPermissionRead
	case "write-all":
		level = actions_model.TokenPermissionWrite
	case "":
		// Empty scalar means use defaults
		return nil, nil //nolint:nilnil
	default:
		return nil, fmt.Errorf("unknown permissions value: %q, expected 'read-all' or 'write-all'", value)
	}

	perms := make(actions_model.TokenPermissions)
	for _, scope := range allKnownScopes() {
		perms[scope] = level
	}
	return perms, nil
}

// parseMappingPermissions handles the mapping form: scope: level
func parseMappingPermissions(node *yaml.Node) (actions_model.TokenPermissions, error) {
	if len(node.Content)%2 != 0 {
		return nil, fmt.Errorf("invalid permissions mapping: odd number of elements")
	}

	perms := make(actions_model.TokenPermissions)
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		value := node.Content[i+1].Value

		// Validate scope
		if _, ok := actions_model.ScopeToUnitType(key); !ok {
			// Accept but ignore unknown scopes for forward compatibility
			continue
		}

		// Validate level
		switch actions_model.TokenPermissionLevel(value) {
		case actions_model.TokenPermissionNone, actions_model.TokenPermissionRead, actions_model.TokenPermissionWrite:
			perms[key] = actions_model.TokenPermissionLevel(value)
		default:
			return nil, fmt.Errorf("invalid permission level %q for scope %q, expected 'none', 'read', or 'write'", value, key)
		}
	}
	return perms, nil
}

// ComputeJobPermissions computes the effective token permissions for a job.
// It considers:
//  1. The repository's default token permission mode (permissive or restricted)
//  2. The workflow-level permissions (if any)
//  3. The job-level permissions (which override workflow-level)
//
// The repo max settings act as a ceiling - workflow/job permissions cannot exceed them.
func ComputeJobPermissions(
	repoConfig *repo_model.ActionsConfig,
	workflowPermissions *yaml.Node,
	jobPermissions *yaml.Node,
	isForkPullRequest bool,
) (actions_model.TokenPermissions, error) {
	// Step 1: Start with default permissions based on repo config
	defaults := getDefaultPermissions(repoConfig)

	// Step 2: If this is a fork pull request, enforce read-only
	if isForkPullRequest {
		for scope := range defaults {
			if defaults[scope] > actions_model.TokenPermissionRead {
				defaults[scope] = actions_model.TokenPermissionRead
			}
		}
	}

	// Step 3: Parse workflow-level permissions
	wfPerms, err := ParsePermissionsFromYAMLNode(workflowPermissions)
	if err != nil {
		return nil, fmt.Errorf("parse workflow permissions: %w", err)
	}

	// Step 4: Parse job-level permissions
	jobPerms, err := ParsePermissionsFromYAMLNode(jobPermissions)
	if err != nil {
		return nil, fmt.Errorf("parse job permissions: %w", err)
	}

	// Step 5: Determine effective permissions
	// Priority: job > workflow > defaults
	effective := defaults
	if wfPerms != nil {
		effective = applyPermissions(effective, wfPerms)
	}
	if jobPerms != nil {
		effective = applyPermissions(effective, jobPerms)
	}

	// Step 6: Clamp by repo max settings
	maxPerms := getMaxPermissions(repoConfig, isForkPullRequest)
	effective = clampPermissions(effective, maxPerms)

	return effective, nil
}

// getDefaultPermissions returns the default permissions based on the repo's ActionsConfig.
// In "permissive" mode (default), all scopes get write access.
// In "restricted" mode, only contents and packages get read access, others get none.
func getDefaultPermissions(cfg *repo_model.ActionsConfig) actions_model.TokenPermissions {
	perms := make(actions_model.TokenPermissions)

	if cfg != nil && cfg.DefaultTokenPermission == repo_model.ActionsTokenPermissionRestricted {
		// Restricted mode: read for contents and packages, none for others
		for _, scope := range allKnownScopes() {
			switch scope {
			case "contents", "packages":
				perms[scope] = actions_model.TokenPermissionRead
			default:
				perms[scope] = actions_model.TokenPermissionNone
			}
		}
	} else {
		// Permissive mode (default): write for all scopes
		for _, scope := range allKnownScopes() {
			perms[scope] = actions_model.TokenPermissionWrite
		}
	}

	return perms
}

// getMaxPermissions returns the maximum permissions allowed by the repo configuration.
func getMaxPermissions(cfg *repo_model.ActionsConfig, isForkPullRequest bool) actions_model.TokenPermissions {
	maxPerms := make(actions_model.TokenPermissions)

	if isForkPullRequest {
		// Fork PRs are always capped at read
		for _, scope := range allKnownScopes() {
			maxPerms[scope] = actions_model.TokenPermissionRead
		}
		return maxPerms
	}

	// Default max is write for all scopes
	for _, scope := range allKnownScopes() {
		maxPerms[scope] = actions_model.TokenPermissionWrite
	}
	return maxPerms
}

// applyPermissions applies overrides on top of base permissions.
// When overrides specify a scope, that scope is set to the override value.
// Scopes not mentioned in overrides are set to "none" (GitHub behavior:
// specifying any permission in the map means unmentioned scopes default to none).
func applyPermissions(base, overrides actions_model.TokenPermissions) actions_model.TokenPermissions {
	result := make(actions_model.TokenPermissions)
	// When permissions are explicitly specified, unmentioned scopes get "none"
	for _, scope := range allKnownScopes() {
		if level, ok := overrides[scope]; ok {
			result[scope] = level
		} else {
			result[scope] = actions_model.TokenPermissionNone
		}
	}
	return result
}

// clampPermissions ensures no permission exceeds the maximum allowed level
func clampPermissions(perms, maxPerms actions_model.TokenPermissions) actions_model.TokenPermissions {
	result := make(actions_model.TokenPermissions)
	for scope, level := range perms {
		maxLevel, ok := maxPerms[scope]
		if !ok {
			maxLevel = actions_model.TokenPermissionWrite
		}
		result[scope] = minPermissionLevel(level, maxLevel)
	}
	return result
}

// minPermissionLevel returns the lower of two permission levels
func minPermissionLevel(a, b actions_model.TokenPermissionLevel) actions_model.TokenPermissionLevel {
	aMode := actions_model.PermissionLevelToAccessMode(a)
	bMode := actions_model.PermissionLevelToAccessMode(b)
	if aMode < bMode {
		return a
	}
	return b
}

// ComputeJobPermissionsFromWorkflowPayload computes permissions from a job's workflow payload.
// This is used when creating the task from the stored job.
func ComputeJobPermissionsFromWorkflowPayload(
	repoConfig *repo_model.ActionsConfig,
	workflowPayload []byte,
	isForkPullRequest bool,
) (actions_model.TokenPermissions, error) {
	workflows, err := jobparser.Parse(workflowPayload)
	if err != nil {
		return nil, fmt.Errorf("parse workflow payload: %w", err)
	}
	if len(workflows) != 1 {
		return nil, fmt.Errorf("expected exactly one workflow, got %d", len(workflows))
	}

	sw := workflows[0]
	_, job := sw.Job()
	if job == nil {
		return nil, fmt.Errorf("no job found in workflow payload")
	}

	return ComputeJobPermissions(
		repoConfig,
		&sw.RawPermissions,
		&job.RawPermissions,
		isForkPullRequest,
	)
}
