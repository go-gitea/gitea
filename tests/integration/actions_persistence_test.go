// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsTokenPermissionsPersistence(t *testing.T) {
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	session := loginUser(t, user2.Name)

	// 1. Enable Max Permissions
	req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/token_permissions", repo.OwnerName, repo.Name), map[string]string{
		"token_permission_mode":  "permissive",
		"override_owner_config":  "true",
		"enable_max_permissions": "true",
		"max_unit_access_mode_" + strconv.Itoa(int(unit_model.TypeCode)): "read",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	// Verify
	repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
	require.NoError(t, repo.LoadUnits(t.Context()))
	unit, err := repo.GetUnit(t.Context(), unit_model.TypeActions)
	require.NoError(t, err)
	cfg := unit.ActionsConfig()
	require.NotNil(t, cfg.MaxTokenPermissions, "MaxTokenPermissions should NOT be nil")
	assert.Equal(t, "read", cfg.MaxTokenPermissions.UnitAccessModes[unit_model.TypeCode].ToString())

	// 2. Disable Max Permissions (Keep Override checked)
	req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/token_permissions", repo.OwnerName, repo.Name), map[string]string{
		"token_permission_mode": "permissive",
		"override_owner_config": "true",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	// Verify
	repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
	require.NoError(t, repo.LoadUnits(t.Context()))
	unit, err = repo.GetUnit(t.Context(), unit_model.TypeActions)
	require.NoError(t, err)
	cfg = unit.ActionsConfig()
	require.Nil(t, cfg.MaxTokenPermissions, "MaxTokenPermissions SHOULD be nil")
}
