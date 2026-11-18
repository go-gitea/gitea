// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/require"
)

func TestDeleteUserResourcesOnlyCleansPersonalResources(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	userID := int64(2)
	userManager := insertServiceManager(t)
	userManager.UserID = userID
	userManager.GatewayURL = "https://user-delete.example.com"
	_, err := db.GetEngine(t.Context()).ID(userManager.ID).Cols("user_id", "gateway_url").Update(userManager)
	require.NoError(t, err)

	ownedID := int64(1138)
	insertServiceCodespace(t, userManager.ID, &codespace_model.Codespace{
		ID:                ownedID,
		UserID:            userID,
		RepoID:            2,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 67,
	})
	repositoryOwnedID := int64(1139)
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		ID:                repositoryOwnedID,
		UserID:            1,
		RepoID:            1,
		Status:            codespace_model.StatusStopped,
		OperationRVersion: 68,
	})
	unassigned := &codespace_model.Codespace{
		UserID: userID,
		RepoID: 2,
		Status: codespace_model.StatusFailed,
	}
	insertServiceCodespace(t, 0, unassigned)
	authorization := &codespace_model.PermissionAuthorization{
		UserID: userID, SourceRepoID: 1, RequestHash: "user-delete",
		CreatedUnix: 1, UpdatedUnix: 1,
	}
	require.NoError(t, db.Insert(t.Context(), authorization))
	rule := &codespace_model.PermissionRepository{
		AuthorizationID: authorization.ID, TargetRepoID: 2, UnitType: unit.TypeCode,
		RequestedMode: perm.AccessModeRead, GrantedMode: perm.AccessModeRead,
	}
	require.NoError(t, db.Insert(t.Context(), rule))
	secret := &codespace_model.UserSecret{UserID: userID, Name: "DATABASE_PASSWORD", DataEncrypted: "encrypted"}
	require.NoError(t, db.Insert(t.Context(), secret))
	registrationToken := &codespace_model.ManagerRegistrationToken{UserID: userID, Token: "1111111111111111111111111111111111111111", IsActive: true}
	require.NoError(t, db.Insert(t.Context(), registrationToken))

	require.NoError(t, DeleteUserResources(t.Context(), userID))

	assertServiceNotExists(t, new(codespace_model.Manager), "id = ?", userManager.ID)
	assertServiceNotExists(t, new(codespace_model.Codespace), "id = ?", ownedID)
	assertServiceNotExists(t, new(codespace_model.Codespace), "id = ?", unassigned.ID)
	assertServiceExists(t, new(codespace_model.Codespace), "id = ?", repositoryOwnedID)
	assertServiceNotExists(t, new(codespace_model.PermissionAuthorization), "id = ?", authorization.ID)
	assertServiceNotExists(t, new(codespace_model.PermissionRepository), "authorization_id = ? AND target_repo_id = ? AND unit_type = ?", rule.AuthorizationID, rule.TargetRepoID, rule.UnitType)
	assertServiceNotExists(t, new(codespace_model.UserSecret), "id = ?", secret.ID)
	assertServiceNotExists(t, new(codespace_model.ManagerRegistrationToken), "id = ?", registrationToken.ID)
}
