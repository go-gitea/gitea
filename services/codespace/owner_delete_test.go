// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/require"
)

func TestDeleteOwnerResourcesCleansOwnerScope(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	ownerID := int64(3)
	ownerToken, err := GetOrCreateRegistrationToken(t.Context(), ManagerSettingsOptions{
		Scope:   ManagerSettingsScopeOrganization,
		OwnerID: ownerID,
	})
	require.NoError(t, err)
	globalToken, err := GetOrCreateRegistrationToken(t.Context(), ManagerSettingsOptions{Scope: ManagerSettingsScopeSite})
	require.NoError(t, err)

	ownerManager := insertServiceManager(t)
	ownerManager.OwnerID = ownerID
	require.NoError(t, updateManagerOwner(t, ownerManager))
	globalManager := insertServiceManager(t)
	insertSettingsManagerAddress(t, ownerManager.ID, codespace_model.ManagerAddressGateway, "https://owner-delete.example.com")

	managerBoundUUID := "61616161-6161-4161-8161-616161616161"
	insertServiceCodespace(t, ownerManager.ID, &codespace_model.Codespace{
		UUID:              managerBoundUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 61,
	})
	insertServiceCredentials(t, managerBoundUUID)

	repoOwnedUUID := "62626262-6262-4262-8262-626262626262"
	insertServiceCodespace(t, globalManager.ID, &codespace_model.Codespace{
		UUID:              repoOwnedUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 62,
	})
	require.NoError(t, updateServiceCodespaceOwnerFields(t, repoOwnedUUID, 1, 32))
	insertServiceCredentials(t, repoOwnedUUID)

	unrelatedUUID := "63636363-6363-4363-8363-636363636363"
	insertServiceCodespace(t, globalManager.ID, &codespace_model.Codespace{
		UUID:              unrelatedUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 63,
	})
	insertServiceCredentials(t, unrelatedUUID)

	require.NoError(t, DeleteOwnerResources(t.Context(), ownerID))

	assertServiceNotExists(t, new(codespace_model.Manager), "id = ?", ownerManager.ID)
	assertServiceNotExists(t, new(codespace_model.ManagerAddress), "manager_id = ?", ownerManager.ID)
	assertServiceNotExists(t, new(codespace_model.Codespace), "uuid = ?", managerBoundUUID)
	assertServiceNotExists(t, new(codespace_model.Codespace), "uuid = ?", repoOwnedUUID)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", managerBoundUUID)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", repoOwnedUUID)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", managerBoundUUID)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", repoOwnedUUID)
	assertServiceNotExists(t, new(codespace_model.ManagerToken), "owner_id = ? AND token = ?", ownerID, ownerToken)

	assertServiceExists(t, new(codespace_model.Manager), "id = ?", globalManager.ID)
	assertServiceExists(t, new(codespace_model.Codespace), "uuid = ?", unrelatedUUID)
	assertServiceExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", unrelatedUUID)
	assertServiceExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", unrelatedUUID)
	assertServiceExists(t, new(codespace_model.ManagerToken), "owner_id = ? AND token = ?", 0, globalToken)
}

func updateManagerOwner(t *testing.T, manager *codespace_model.Manager) error {
	t.Helper()
	_, err := db.GetEngine(t.Context()).ID(manager.ID).Cols("owner_id").Update(manager)
	return err
}

func updateServiceCodespaceOwnerFields(t *testing.T, codespaceUUID string, userID, repoID int64) error {
	t.Helper()
	_, err := db.GetEngine(t.Context()).ID(codespaceUUID).Cols("user_id", "repo_id").Update(&codespace_model.Codespace{
		UserID: userID,
		RepoID: repoID,
	})
	return err
}
