// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
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

	require.ErrorIs(t, ReducePermissionRepository(t.Context(), 4, authorization.ID, rule.ID, perm.AccessModeRead), ErrPermissionAuthorizationNotFound)
	require.ErrorIs(t, RevokePermissionAuthorization(t.Context(), 4, authorization.ID), ErrPermissionAuthorizationNotFound)
	require.ErrorIs(t, ReducePermissionRepository(t.Context(), 2, authorization.ID, otherRule.ID, perm.AccessModeRead), ErrPermissionAuthorizationNotFound)

	require.NoError(t, ReducePermissionRepository(t.Context(), 2, authorization.ID, rule.ID, perm.AccessModeRead))
	rule = unittest.AssertExistsAndLoadBean(t, &codespace_model.PermissionRepository{ID: rule.ID})
	assert.Equal(t, perm.AccessModeRead, rule.GrantedMode)
	require.NoError(t, ReducePermissionRepository(t.Context(), 2, authorization.ID, rule.ID, perm.AccessModeNone))
	require.ErrorIs(t, ReducePermissionRepository(t.Context(), 2, authorization.ID, rule.ID, perm.AccessModeRead), ErrPermissionReductionInvalid)
	rule = unittest.AssertExistsAndLoadBean(t, &codespace_model.PermissionRepository{ID: rule.ID})
	assert.Equal(t, perm.AccessModeNone, rule.GrantedMode)

	require.NoError(t, RevokePermissionAuthorization(t.Context(), 2, authorization.ID))
	authorization = unittest.AssertExistsAndLoadBean(t, &codespace_model.PermissionAuthorization{ID: authorization.ID})
	assert.Positive(t, authorization.RevokedUnix)
	require.ErrorIs(t, ReducePermissionRepository(t.Context(), 2, authorization.ID, rule.ID, perm.AccessModeNone), ErrPermissionAuthorizationNotFound)
}
