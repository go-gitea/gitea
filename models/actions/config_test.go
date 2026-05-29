// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"

	"github.com/stretchr/testify/assert"
)

func TestOwnerActionsConfigTokenPermissions(t *testing.T) {
	t.Run("Zero Value Permission Mode", func(t *testing.T) {
		cfg := &OwnerActionsConfig{}
		assert.Equal(t, perm.AccessModeWrite, cfg.GetDefaultTokenPermissions().UnitAccessModes[unit.TypeCode])
	})

	t.Run("Restricted Permission Mode", func(t *testing.T) {
		cfg := &OwnerActionsConfig{
			TokenPermissionMode: repo_model.ActionsTokenPermissionModeRestricted,
		}
		defaultPerms := cfg.GetDefaultTokenPermissions()
		assert.Equal(t, perm.AccessModeRead, defaultPerms.UnitAccessModes[unit.TypeCode])
		assert.Equal(t, perm.AccessModeNone, defaultPerms.UnitAccessModes[unit.TypeIssues])
		assert.Equal(t, perm.AccessModeRead, defaultPerms.UnitAccessModes[unit.TypePackages])
	})
}
