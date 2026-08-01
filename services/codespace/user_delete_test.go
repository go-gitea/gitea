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
	userToken, err := GetOrCreateRegistrationToken(t.Context(), ManagerSettingsOptions{
		Scope:  ManagerSettingsScopeUser,
		UserID: userID,
	})
	require.NoError(t, err)
	globalToken, err := GetOrCreateRegistrationToken(t.Context(), ManagerSettingsOptions{Scope: ManagerSettingsScopeSite})
	require.NoError(t, err)

	userManager := insertServiceManager(t)
	userManager.UserID = userID
	_, err = db.GetEngine(t.Context()).ID(userManager.ID).Cols("user_id").Update(userManager)
	require.NoError(t, err)
	insertSettingsManagerAddress(t, userManager.ID, codespace_model.ManagerAddressGateway, "https://user-delete.example.com")

	ownedUUID := "67676767-6767-4767-8767-676767676767"
	insertServiceCodespace(t, userManager.ID, &codespace_model.Codespace{
		UUID:              ownedUUID,
		UserID:            userID,
		RepoID:            2,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 67,
	})
	repositoryOwnedUUID := "68686868-6868-4868-8868-686868686868"
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		UUID:              repositoryOwnedUUID,
		UserID:            1,
		RepoID:            1,
		Status:            codespace_model.StatusStopped,
		OperationRVersion: 68,
	})
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
	secret := &codespace_model.UserSecret{UserID: userID, Name: "DATABASE_PASSWORD", DataEncrypted: "encrypted", DataSize: 8}
	require.NoError(t, db.Insert(t.Context(), secret))
	secretRepository := &codespace_model.UserSecretRepository{SecretID: secret.ID, RepoID: 2}
	require.NoError(t, db.Insert(t.Context(), secretRepository))

	require.NoError(t, DeleteUserResources(t.Context(), userID))

	assertServiceNotExists(t, new(codespace_model.Manager), "id = ?", userManager.ID)
	assertServiceNotExists(t, new(codespace_model.ManagerAddress), "manager_id = ?", userManager.ID)
	assertServiceNotExists(t, new(codespace_model.Codespace), "uuid = ?", ownedUUID)
	assertServiceExists(t, new(codespace_model.Codespace), "uuid = ?", repositoryOwnedUUID)
	assertServiceNotExists(t, new(codespace_model.ManagerToken), "user_id = ? AND token = ?", userID, userToken)
	assertServiceExists(t, new(codespace_model.ManagerToken), "user_id = ? AND token = ?", 0, globalToken)
	assertServiceNotExists(t, new(codespace_model.PermissionAuthorization), "id = ?", authorization.ID)
	assertServiceNotExists(t, new(codespace_model.PermissionRepository), "authorization_id = ? AND target_repo_id = ? AND unit_type = ?", rule.AuthorizationID, rule.TargetRepoID, rule.UnitType)
	assertServiceNotExists(t, new(codespace_model.UserSecret), "id = ?", secret.ID)
	assertServiceNotExists(t, new(codespace_model.UserSecretRepository), "secret_id = ? AND repo_id = ?", secretRepository.SecretID, secretRepository.RepoID)
}
