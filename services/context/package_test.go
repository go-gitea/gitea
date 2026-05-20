// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"net/http"
	"net/http/httptest"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetermineAccessModeActionsUser(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	req := httptest.NewRequest(http.MethodGet, "/api/packages/user2/generic/pkg/1/file", nil)
	base := NewBaseContextForTest(httptest.NewRecorder(), req)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	require.NoError(t, db.Insert(ctx, &repo_model.RepoUnit{
		RepoID: repo.ID,
		Type:   unit.TypeActions,
		Config: &repo_model.ActionsConfig{},
	}))

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	otherOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 31})
	doer := user_model.NewActionsUserWithTaskID(53)
	task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 53})
	require.NoError(t, task.LoadJob(ctx))

	perms := repo_model.MakeActionsTokenPermissions(perm.AccessModeNone)
	perms.UnitAccessModes[unit.TypePackages] = perm.AccessModeRead
	task.Job.TokenPermissions = &perms
	_, err := actions_model.UpdateRunJob(ctx, task.Job, nil, "token_permissions")
	require.NoError(t, err)

	mode, err := determineAccessMode(base, owner, doer)
	require.NoError(t, err)
	assert.Equal(t, perm.AccessModeRead, mode)

	mode, err = determineAccessMode(base, otherOwner, doer)
	require.NoError(t, err)
	assert.Equal(t, perm.AccessModeNone, mode)

	perms.UnitAccessModes[unit.TypePackages] = perm.AccessModeWrite
	task.Job.TokenPermissions = &perms
	_, err = actions_model.UpdateRunJob(ctx, task.Job, nil, "token_permissions")
	require.NoError(t, err)

	mode, err = determineAccessMode(base, owner, doer)
	require.NoError(t, err)
	assert.Equal(t, perm.AccessModeWrite, mode)
}
