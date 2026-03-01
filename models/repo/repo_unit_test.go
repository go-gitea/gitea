// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/perm"

	"github.com/stretchr/testify/assert"
)

func TestActionsConfig(t *testing.T) {
	cfg := &ActionsConfig{}
	cfg.DisableWorkflow("test1.yaml")
	assert.Equal(t, []string{"test1.yaml"}, cfg.DisabledWorkflows)

	cfg.DisableWorkflow("test1.yaml")
	assert.Equal(t, []string{"test1.yaml"}, cfg.DisabledWorkflows)

	cfg.EnableWorkflow("test1.yaml")
	assert.Equal(t, []string{}, cfg.DisabledWorkflows)

	cfg.EnableWorkflow("test1.yaml")
	assert.Equal(t, []string{}, cfg.DisabledWorkflows)

	cfg.DisableWorkflow("test1.yaml")
	cfg.DisableWorkflow("test2.yaml")
	cfg.DisableWorkflow("test3.yaml")
	assert.Equal(t, "test1.yaml,test2.yaml,test3.yaml", cfg.ToString())
}

func TestActionsConfigTokenPermissions(t *testing.T) {
	t.Run("Default Permission Mode", func(t *testing.T) {
		cfg := &ActionsConfig{}
		assert.Equal(t, ActionsTokenPermissionModePermissive, cfg.GetTokenPermissionMode())
	})

	t.Run("Explicit Permission Mode", func(t *testing.T) {
		cfg := &ActionsConfig{
			TokenPermissionMode: ActionsTokenPermissionModeRestricted,
		}
		assert.Equal(t, ActionsTokenPermissionModeRestricted, cfg.GetTokenPermissionMode())
	})

	t.Run("Effective Permissions - Permissive Mode", func(t *testing.T) {
		cfg := &ActionsConfig{
			TokenPermissionMode: ActionsTokenPermissionModePermissive,
		}
		perms := cfg.GetDefaultTokenPermissions()
		assert.Equal(t, perm.AccessModeWrite, perms.Code)
		assert.Equal(t, perm.AccessModeWrite, perms.Issues)
		assert.Equal(t, perm.AccessModeRead, perms.Packages) // Packages read by default for security
	})

	t.Run("Effective Permissions - Restricted Mode", func(t *testing.T) {
		cfg := &ActionsConfig{
			TokenPermissionMode: ActionsTokenPermissionModeRestricted,
		}
		perms := cfg.GetDefaultTokenPermissions()
		assert.Equal(t, perm.AccessModeRead, perms.Code)
		assert.Equal(t, perm.AccessModeNone, perms.Issues)
		assert.Equal(t, perm.AccessModeRead, perms.Packages)
	})

	t.Run("Clamp Permissions", func(t *testing.T) {
		cfg := &ActionsConfig{
			MaxTokenPermissions: &ActionsTokenPermissions{
				Code:         perm.AccessModeRead,
				Issues:       perm.AccessModeWrite,
				PullRequests: perm.AccessModeRead,
				Packages:     perm.AccessModeRead,
				Actions:      perm.AccessModeNone,
				Wiki:         perm.AccessModeWrite,
			},
		}
		input := ActionsTokenPermissions{
			Code:         perm.AccessModeWrite, // Should be clamped to Read
			Issues:       perm.AccessModeWrite, // Should stay Write
			PullRequests: perm.AccessModeWrite, // Should be clamped to Read
			Packages:     perm.AccessModeWrite, // Should be clamped to Read
			Actions:      perm.AccessModeRead,  // Should be clamped to None
			Wiki:         perm.AccessModeRead,  // Should stay Read
		}
		clamped := cfg.ClampPermissions(input)
		assert.Equal(t, perm.AccessModeRead, clamped.Code)
		assert.Equal(t, perm.AccessModeWrite, clamped.Issues)
		assert.Equal(t, perm.AccessModeRead, clamped.PullRequests)
		assert.Equal(t, perm.AccessModeRead, clamped.Packages)
		assert.Equal(t, perm.AccessModeNone, clamped.Actions)
		assert.Equal(t, perm.AccessModeRead, clamped.Wiki)
	})
}
