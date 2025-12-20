// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
)

// EffectivePermissions represents the final calculated permissions for an Actions token
type EffectivePermissions struct {
	// Map structure: resource -> action -> allowed
	// Example: {"contents": {"read": true, "write": false}}
	Permissions map[string]map[string]bool

	// Whether this token is from a fork PR (always restricted)
	IsFromForkPR bool

	// The permission mode used
	Mode actions_model.PermissionMode
}

// PermissionChecker handles all permission checking logic for Actions tokens
type PermissionChecker struct {
	ctx context.Context
}

// NewPermissionChecker creates a new permission checker
func NewPermissionChecker(ctx context.Context) *PermissionChecker {
	return &PermissionChecker{ctx: ctx}
}

// GetEffectivePermissions calculates the final permissions for an Actions workflow
//
// Permission hierarchy (most restrictive wins):
// 1. Fork PR restriction (if applicable) - ALWAYS read-only
// 2. Organization settings (if exists) - caps maximum permissions
// 3. Repository settings (if exists) - further restricts
// 4. Workflow file permissions (if declared) - selects subset
//
// This implements the security model proposed in:
// https://github.com/go-gitea/gitea/issues/24635
func (pc *PermissionChecker) GetEffectivePermissions(
	repoID int64,
	orgID int64,
	isFromForkPR bool,
	workflowPermissions map[string]string, // From workflow YAML
) (*EffectivePermissions, error) {
	// SECURITY: Fork PRs are ALWAYS restricted, regardless of any configuration
	// This prevents malicious PRs from accessing sensitive resources
	// Reference: https://github.com/go-gitea/gitea/pull/24554#issuecomment-1537040811
	if isFromForkPR {
		return &EffectivePermissions{
			Permissions:  getRestrictedPermissions(),
			IsFromForkPR: true,
			Mode:         actions_model.PermissionModeRestricted,
		}, nil
	}

	// Start with repository permissions (or defaults)
	repoPerms, err := pc.getRepoPermissions(repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo permissions: %w", err)
	}

	// Apply organization cap if org exists
	if orgID > 0 {
		orgPerms, err := pc.getOrgPermissions(orgID)
		if err != nil {
			return nil, fmt.Errorf("failed to get org permissions: %w", err)
		}

		// Organization settings cap repository settings
		// Repo can only reduce permissions, never increase beyond org
		repoPerms = capPermissions(repoPerms, orgPerms)
	}

	// Apply workflow file permissions if specified
	// Workflow can select a subset but cannot escalate beyond repo/org
	finalPerms := repoPerms
	if len(workflowPermissions) > 0 {
		finalPerms = applyWorkflowPermissions(repoPerms, workflowPermissions)
	}

	return &EffectivePermissions{
		Permissions:  finalPerms,
		IsFromForkPR: false,
		Mode:         actions_model.PermissionModeCustom, // Effective mode after merging
	}, nil
}

// getRepoPermissions retrieves repository-level permissions or returns defaults
func (pc *PermissionChecker) getRepoPermissions(repoID int64) (map[string]map[string]bool, error) {
	perm, err := actions_model.GetRepoActionPermissions(pc.ctx, repoID)
	if err != nil {
		return nil, err
	}

	if perm == nil {
		// No custom config - use restricted defaults
		return getRestrictedPermissions(), nil
	}

	return perm.ToPermissionMap(), nil
}

// getOrgPermissions retrieves organization-level permissions or returns defaults
func (pc *PermissionChecker) getOrgPermissions(orgID int64) (map[string]map[string]bool, error) {
	perm, err := actions_model.GetOrgActionPermissions(pc.ctx, orgID)
	if err != nil {
		return nil, err
	}

	if perm == nil {
		// No custom config - use restricted defaults
		return getRestrictedPermissions(), nil
	}

	return perm.ToPermissionMap(), nil
}

// getRestrictedPermissions returns the default restricted permission set
func getRestrictedPermissions() map[string]map[string]bool {
	return map[string]map[string]bool{
		"actions":       {"read": false, "write": false},
		"contents":      {"read": true, "write": false}, // Can read code
		"issues":        {"read": false, "write": false},
		"packages":      {"read": false, "write": false},
		"pull_requests": {"read": false, "write": false},
		"metadata":      {"read": true, "write": false}, // Can read repo metadata
	}
}

// capPermissions applies organizational caps to repository permissions
// Returns the more restrictive of the two permission sets
func capPermissions(repoPerms, orgPerms map[string]map[string]bool) map[string]map[string]bool {
	result := make(map[string]map[string]bool)

	for resource, actions := range repoPerms {
		result[resource] = make(map[string]bool)

		for action, repoAllowed := range actions {
			orgAllowed := false
			if orgActions, ok := orgPerms[resource]; ok {
				orgAllowed = orgActions[action]
			}

			// Use the MORE restrictive (logical AND)
			// If either org or repo denies, final result is deny
			result[resource][action] = repoAllowed && orgAllowed
		}
	}

	return result
}

// applyWorkflowPermissions applies workflow file permission declarations
// Workflow can only select a subset, cannot escalate
func applyWorkflowPermissions(basePerms map[string]map[string]bool, workflowPerms map[string]string) map[string]map[string]bool {
	result := make(map[string]map[string]bool)

	for resource := range basePerms {
		result[resource] = make(map[string]bool)

		// Check if workflow declares this resource
		workflowPerm, declared := workflowPerms[resource]
		if !declared {
			// Not declared in workflow - use base permissions
			result[resource] = basePerms[resource]
			continue
		}

		// Workflow declared this resource - apply restrictions
		switch workflowPerm {
		case "none":
			// Workflow explicitly denies
			result[resource]["read"] = false
			result[resource]["write"] = false

		case "read":
			// Workflow wants read - but only if base allows
			result[resource]["read"] = basePerms[resource]["read"]
			result[resource]["write"] = false

		case "write":
			// Workflow wants write - but only if base allows both read and write
			// (write implies read in GitHub's model)
			result[resource]["read"] = basePerms[resource]["read"]
			result[resource]["write"] = basePerms[resource]["write"]

		default:
			// Unknown permission level - deny
			result[resource]["read"] = false
			result[resource]["write"] = false
		}
	}

	return result
}

// CheckPermission checks if a specific action is allowed
func (ep *EffectivePermissions) CheckPermission(resource, action string) bool {
	if ep.Permissions == nil {
		return false
	}

	if actions, ok := ep.Permissions[resource]; ok {
		return actions[action]
	}

	return false
}

// CanRead checks if reading a resource is allowed
func (ep *EffectivePermissions) CanRead(resource string) bool {
	return ep.CheckPermission(resource, "read")
}

// CanWrite checks if writing to a resource is allowed
func (ep *EffectivePermissions) CanWrite(resource string) bool {
	return ep.CheckPermission(resource, "write")
}

// ToTokenClaims converts permissions to JWT claims format
func (ep *EffectivePermissions) ToTokenClaims() map[string]interface{} {
	claims := make(map[string]interface{})

	// Add permissions map
	claims["permissions"] = ep.Permissions

	// Add fork PR flag
	claims["is_fork_pr"] = ep.IsFromForkPR

	// Add permission mode
	claims["permission_mode"] = int(ep.Mode)

	return claims
}

// ParsePermissionsFromClaims extracts permissions from JWT token claims
func ParsePermissionsFromClaims(claims map[string]interface{}) *EffectivePermissions {
	ep := &EffectivePermissions{
		Permissions: make(map[string]map[string]bool),
	}

	// Extract permissions map
	if perms, ok := claims["permissions"].(map[string]interface{}); ok {
		for resource, actions := range perms {
			ep.Permissions[resource] = make(map[string]bool)
			if actionMap, ok := actions.(map[string]interface{}); ok {
				for action, allowed := range actionMap {
					if allowedBool, ok := allowed.(bool); ok {
						ep.Permissions[resource][action] = allowedBool
					}
				}
			}
		}
	}

	// Extract fork PR flag
	if isForkPR, ok := claims["is_fork_pr"].(bool); ok {
		ep.IsFromForkPR = isForkPR
	}

	// Extract permission mode
	if mode, ok := claims["permission_mode"].(float64); ok {
		ep.Mode = actions_model.PermissionMode(int(mode))
	}

	return ep
}
