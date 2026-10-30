// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"slices"
	"testing"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReduceAndRevokePermissionAuthorization(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	now := time.Now().Unix()
	authorization := &codespace_model.PermissionAuthorization{
		UserID: 2, SourceRepoID: 1, RequestHash: "request",
		CreatedUnix: now, UpdatedUnix: now,
	}
	require.NoError(t, db.Insert(t.Context(), authorization))
	rule := &codespace_model.PermissionRepository{
		AuthorizationID: authorization.ID,
		TargetRepoID:    2,
		UnitType:        unit.TypeCode,
		RequestedMode:   perm.AccessModeWrite,
		GrantedMode:     perm.AccessModeWrite,
	}
	require.NoError(t, db.Insert(t.Context(), rule))
	otherAuthorization := &codespace_model.PermissionAuthorization{
		UserID: 2, SourceRepoID: 3, RequestHash: "other-request",
		CreatedUnix: now, UpdatedUnix: now,
	}
	require.NoError(t, db.Insert(t.Context(), otherAuthorization))
	otherRule := &codespace_model.PermissionRepository{
		AuthorizationID: otherAuthorization.ID,
		TargetRepoID:    1,
		UnitType:        unit.TypeCode,
		RequestedMode:   perm.AccessModeWrite,
		GrantedMode:     perm.AccessModeWrite,
	}
	require.NoError(t, db.Insert(t.Context(), otherRule))
	require.NoError(t, db.Insert(t.Context(), &codespace_model.PermissionAuthorization{
		UserID: 4, SourceRepoID: 1, RequestHash: "another-user",
		CreatedUnix: now, UpdatedUnix: now,
	}))
	require.NoError(t, db.Insert(t.Context(), &codespace_model.PermissionRepository{
		AuthorizationID: authorization.ID, TargetRepoID: 99999, UnitType: unit.TypeCode,
		RequestedMode: perm.AccessModeRead, GrantedMode: perm.AccessModeRead,
	}))
	require.NoError(t, db.Insert(t.Context(), &codespace_model.Codespace{
		UUID: "85858585-8585-4858-8858-858585858585", UserID: 2, RepoID: 1,
		PermissionAuthorizationID: authorization.ID, Status: codespace_model.StatusRunning,
	}))

	views, err := ListPermissionAuthorizations(t.Context(), 2)
	require.NoError(t, err)
	require.Len(t, views, 2)
	assert.Equal(t, otherAuthorization.ID, views[0].ID)
	assert.Zero(t, views[0].CodespaceCount)
	view := views[1]
	assert.Equal(t, authorization.ID, view.ID)
	assert.Equal(t, "user2/repo1", view.SourceRepository)
	assert.EqualValues(t, 1, view.CodespaceCount)
	require.Len(t, view.Repositories, 1)
	assert.Equal(t, PermissionRepositoryView{
		TargetRepoID: 2, UnitType: unit.TypeCode, Repository: "user2/repo2", UnitName: "code",
		RequestedMode: "write", GrantedMode: "write", CanReduceToRead: true, CanRevoke: true,
	}, view.Repositories[0])

	require.ErrorIs(t, ReducePermissionRepository(t.Context(), 4, authorization.ID, rule.TargetRepoID, rule.UnitType, perm.AccessModeRead), ErrPermissionAuthorizationNotFound)
	require.ErrorIs(t, RevokePermissionAuthorization(t.Context(), 4, authorization.ID), ErrPermissionAuthorizationNotFound)
	require.ErrorIs(t, ReducePermissionRepository(t.Context(), 2, authorization.ID, otherRule.TargetRepoID, otherRule.UnitType, perm.AccessModeRead), ErrPermissionAuthorizationNotFound)

	require.NoError(t, ReducePermissionRepository(t.Context(), 2, authorization.ID, rule.TargetRepoID, rule.UnitType, perm.AccessModeRead))
	loadedRule := new(codespace_model.PermissionRepository)
	has, err := db.GetEngine(t.Context()).
		Where("authorization_id = ? AND target_repo_id = ? AND unit_type = ?", authorization.ID, rule.TargetRepoID, rule.UnitType).
		Get(loadedRule)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, perm.AccessModeRead, loadedRule.GrantedMode)
	require.NoError(t, ReducePermissionRepository(t.Context(), 2, authorization.ID, rule.TargetRepoID, rule.UnitType, perm.AccessModeNone))
	require.ErrorIs(t, ReducePermissionRepository(t.Context(), 2, authorization.ID, rule.TargetRepoID, rule.UnitType, perm.AccessModeRead), ErrPermissionReductionInvalid)
	loadedRule = new(codespace_model.PermissionRepository)
	has, err = db.GetEngine(t.Context()).
		Where("authorization_id = ? AND target_repo_id = ? AND unit_type = ?", authorization.ID, rule.TargetRepoID, rule.UnitType).
		Get(loadedRule)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, perm.AccessModeNone, loadedRule.GrantedMode)

	require.NoError(t, RevokePermissionAuthorization(t.Context(), 2, authorization.ID))
	authorization = unittest.AssertExistsAndLoadBean(t, &codespace_model.PermissionAuthorization{ID: authorization.ID})
	assert.Positive(t, authorization.RevokedUnix)
	views, err = ListPermissionAuthorizations(t.Context(), 2)
	require.NoError(t, err)
	require.Len(t, views, 2)
	index := slices.IndexFunc(views, func(view *PermissionAuthorizationView) bool { return view.ID == authorization.ID })
	require.GreaterOrEqual(t, index, 0)
	view = views[index]
	assert.True(t, view.Revoked)
	assert.Zero(t, view.CodespaceCount)
	require.Len(t, view.Repositories, 1)
	assert.Equal(t, "none", view.Repositories[0].GrantedMode)
	assert.False(t, view.Repositories[0].CanReduceToRead)
	assert.False(t, view.Repositories[0].CanRevoke)
	require.ErrorIs(t, ReducePermissionRepository(t.Context(), 2, authorization.ID, rule.TargetRepoID, rule.UnitType, perm.AccessModeNone), ErrPermissionAuthorizationNotFound)
}
