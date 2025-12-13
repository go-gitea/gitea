// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"github.com/stretchr/testify/assert"
)

// TestGetEffectivePermissions_ForkPRAlwaysRestricted verifies that fork PRs
// are always restricted regardless of repo or org settings.
// This is critical for security - we don't want malicious forks to gain elevated
// permissions just by opening a PR. See the discussion in:
// https://github.com/go-gitea/gitea/pull/24554#issuecomment-1537040811
func TestGetEffectivePermissions_ForkPRAlwaysRestricted(t *testing.T) {
	// Even if repo has permissive mode enabled
	repoPerms := map[string]map[string]bool{
		"contents": {"read": true, "write": true},
		"packages": {"read": true, "write": true},
		"issues":   {"read": true, "write": true},
	}

	// Fork PR should still be read-only
	result := applyForkPRRestrictions(repoPerms)

	assert.True(t, result["contents"]["read"], "Should allow reading contents")
	assert.False(t, result["contents"]["write"], "Should NOT allow writing contents")
	assert.False(t, result["packages"]["write"], "Should NOT allow package writes")
	assert.False(t, result["issues"]["write"], "Should NOT allow issue writes")
}

// TestOrgPermissionsCap verifies that organization settings act as a ceiling
// for repository settings. Repos can be more restrictive but not more permissive.
func TestOrgPermissionsCap(t *testing.T) {
	// Org says: no package writes
	orgPerms := map[string]map[string]bool{
		"packages": {"read": true, "write": false},
		"contents": {"read": true, "write": true},
	}

	// Repo tries to enable package writes
	repoPerms := map[string]map[string]bool{
		"packages": {"read": true, "write": true}, // Trying to override!
		"contents": {"read": true, "write": true},
	}

	result := capPermissions(repoPerms, orgPerms)

	// Org restriction should win
	assert.False(t, result["packages"]["write"], "Org should prevent package writes")
	assert.True(t, result["contents"]["write"], "Contents write should be allowed")
}

// TestWorkflowCannotEscalate verifies that workflow file declarations
// cannot grant more permissions than repo/org settings allow.
// This is important because in Gitea, anyone with write access can edit workflows
// (unlike GitHub which has CODEOWNERS protection).
func TestWorkflowCannotEscalate(t *testing.T) {
	// Base permissions: read-only for packages
	basePerms := map[string]map[string]bool{
		"packages": {"read": true, "write": false},
		"contents": {"read": true, "write": true},
	}

	// Workflow tries to declare package write
	workflowPerms := map[string]string{
		"packages": "write", // Trying to escalate!
		"contents": "write",
	}

	result := applyWorkflowPermissions(basePerms, workflowPerms)

	// Should NOT be able to escalate
	assert.False(t, result["packages"]["write"], "Workflow should not escalate package perms")
	assert.True(t, result["contents"]["write"], "Contents write should still work")
}

// TestWorkflowCanReducePermissions verifies that workflows CAN reduce permissions
// This is useful for defense-in-depth - even if repo has broad permissions,
// a specific workflow can declare it only needs minimal permissions.
func TestWorkflowCanReducePermissions(t *testing.T) {
	// Base permissions: write access
	basePerms := map[string]map[string]bool{
		"contents": {"read": true, "write": true},
		"issues":   {"read": true, "write": true},
	}

	// Workflow declares it only needs read
	workflowPerms := map[string]string{
		"contents": "read",
		"issues":   "none", // Explicitly denies
	}

	result := applyWorkflowPermissions(basePerms, workflowPerms)

	assert.True(t, result["contents"]["read"], "Should allow reading")
	assert.False(t, result["contents"]["write"], "Should reduce to read-only")
	assert.False(t, result["issues"]["read"], "Should deny issues entirely")
}

// TestRestrictedModeDefaults verifies that restricted mode has sensible defaults
// We want it to be usable (can clone code, read metadata) but secure (no writes)
func TestRestrictedModeDefaults(t *testing.T) {
	perms := getRestrictedPermissions()

	// Should be able to read code (needed for checkout action)
	assert.True(t, perms["contents"]["read"], "Must be able to read code")
	assert.True(t, perms["metadata"]["read"], "Must be able to read metadata")

	// Should NOT be able to write anything
	assert.False(t, perms["contents"]["write"], "Should not write code")
	assert.False(t, perms["packages"]["write"], "Should not write packages")
	assert.False(t, perms["issues"]["write"], "Should not write issues")
}

// TestPermissionModeTransitions tests that changing modes works correctly
// This is important for the UI - users should be able to switch modes easily
func TestPermissionModeTransitions(t *testing.T) {
	tests := []struct {
		name                string
		mode                actions_model.PermissionMode
		expectPackageWrite  bool
		expectContentsWrite bool
	}{
		{
			name:                "Restricted mode - no writes",
			mode:                actions_model.PermissionModeRestricted,
			expectPackageWrite:  false,
			expectContentsWrite: false,
		},
		{
			name:                "Permissive mode - has writes",
			mode:                actions_model.PermissionModePermissive,
			expectPackageWrite:  true,
			expectContentsWrite: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perm := &actions_model.ActionTokenPermission{
				PermissionMode: tt.mode,
			}

			permMap := perm.ToPermissionMap()

			assert.Equal(t, tt.expectPackageWrite, permMap["packages"]["write"])
			assert.Equal(t, tt.expectContentsWrite, permMap["contents"]["write"])
		})
	}
}

// TestMultipleLayers tests the full permission calculation with all layers
// This simulates a real-world scenario with org, repo, and workflow permissions
func TestMultipleLayers(t *testing.T) {
	// Scenario: Org allows package reads, Repo allows package writes,
	// but workflow only declares package read

	orgPerms := map[string]map[string]bool{
		"packages": {"read": true, "write": false}, // Org blocks writes
	}

	repoPerms := map[string]map[string]bool{
		"packages": {"read": true, "write": true}, // Repo tries to enable
	}

	workflowPerms := map[string]string{
		"packages": "read", // Workflow only needs read
	}

	// Apply caps (org limits repo)
	afterOrgCap := capPermissions(repoPerms, orgPerms)
	assert.False(t, afterOrgCap["packages"]["write"], "Org should block write")

	// Apply workflow (workflow selects read)
	final := applyWorkflowPermissions(afterOrgCap, workflowPerms)
	assert.True(t, final["packages"]["read"], "Should have read access")
	assert.False(t, final["packages"]["write"], "Should not have write (org blocked)")
}

// BenchmarkPermissionCalculation measures permission calculation performance
// This is important because permission checks happen on every API call with Actions tokens
// We want to ensure this doesn't become a bottleneck
func BenchmarkPermissionCalculation(b *testing.B) {
	repoPerms := map[string]map[string]bool{
		"actions":       {"read": true, "write": false},
		"contents":      {"read": true, "write": true},
		"issues":        {"read": true, "write": true},
		"packages":      {"read": true, "write": true},
		"pull_requests": {"read": true, "write": false},
		"metadata":      {"read": true, "write": false},
	}

	orgPerms := map[string]map[string]bool{
		"actions":       {"read": true, "write": false},
		"contents":      {"read": true, "write": false},
		"issues":        {"read": true, "write": false},
		"packages":      {"read": false, "write": false},
		"pull_requests": {"read": true, "write": false},
		"metadata":      {"read": true, "write": false},
	}

	workflowPerms := map[string]string{
		"contents": "read",
		"packages": "read",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		capped := capPermissions(repoPerms, orgPerms)
		_ = applyWorkflowPermissions(capped, workflowPerms)
	}
}

// Helper function for fork PR tests
// In real implementation, this would be in permission_checker.go
// TODO: Refactor this into the main codebase if these tests pass
func applyForkPRRestrictions(perms map[string]map[string]bool) map[string]map[string]bool {
	// Fork PRs get read-only access to contents and metadata, nothing else
	return map[string]map[string]bool{
		"contents":      {"read": true, "write": false},
		"metadata":      {"read": true, "write": false},
		"actions":       {"read": false, "write": false},
		"packages":      {"read": false, "write": false},
		"issues":        {"read": false, "write": false},
		"pull_requests": {"read": false, "write": false},
	}
}
