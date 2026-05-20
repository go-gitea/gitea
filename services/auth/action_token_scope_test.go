// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/reqctx"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetActionTokenScope(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	require.NoError(t, db.Insert(ctx, &repo_model.RepoUnit{
		RepoID: repo.ID,
		Type:   unit.TypeActions,
		Config: &repo_model.ActionsConfig{},
	}))

	task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 53})
	require.NoError(t, task.LoadJob(ctx))

	perms := repo_model.MakeActionsTokenPermissions(perm.AccessModeNone)
	perms.UnitAccessModes[unit.TypePackages] = perm.AccessModeRead
	task.Job.TokenPermissions = &perms
	_, err := actions_model.UpdateRunJob(ctx, task.Job, nil, "token_permissions")
	require.NoError(t, err)

	store := reqctx.ContextData{}
	require.NoError(t, setActionTokenScope(ctx, store, task))

	assert.Equal(t, ActionTokenMethodName, store["LoginMethod"])
	assert.True(t, store["IsApiToken"].(bool))
	assert.Equal(t, auth_model.AccessTokenScopeReadPackage, store["ApiTokenScope"])
}
