// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateManagerReturnsOneTimeSecret(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	opts := ManagerSettingsOptions{Scope: ManagerSettingsScopeUser, UserID: 1}
	result, err := CreateManager(t.Context(), opts)
	require.NoError(t, err)
	require.NotZero(t, result.ManagerID)
	require.NotEmpty(t, result.Secret)
	manager, err := AuthenticateManager(t.Context(), result.ManagerID, result.Secret)
	require.NoError(t, err)
	assert.EqualValues(t, 1, manager.UserID)
	assert.Equal(t, codespace_model.ManagerRuntimeStateRecovering, manager.RuntimeState)
}

func TestListManagerSettingsScopesAndDeleteManager(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	globalManager := insertServiceManager(t)
	globalManager.TagsJSON = `[{"tag":"default","description":"Site environment"}]`
	_, err := db.GetEngine(t.Context()).ID(globalManager.ID).Cols("tags_json").Update(globalManager)
	require.NoError(t, err)
	userManager := insertServiceManager(t)
	userManager.UserID = 1
	userManager.Name = "user-manager"
	userManager.Version = "0.2.0"
	userManager.TagsJSON = `[{"tag":"default","description":"Personal environment"},{"tag":"gpu"}]`
	userManager.GatewaySSHHostKeyAlgorithm = "ssh-ed25519"
	userManager.GatewaySSHHostKeyFingerprintSHA256 = "SHA256:settings"
	userManager.GatewaySSHHostKeyUpdatedUnix = 123
	_, err = db.GetEngine(t.Context()).ID(userManager.ID).Cols(
		"user_id", "name", "version", "tags_json", "gateway_ssh_host_key_algorithm",
		"gateway_ssh_host_key_fingerprint_sha256", "gateway_ssh_host_key_updated_unix",
	).Update(userManager)
	require.NoError(t, err)
	insertSettingsManagerAddress(t, globalManager.ID, codespace_model.ManagerAddressGateway, "https://global-gateway.example.com")
	insertSettingsManagerAddress(t, userManager.ID, codespace_model.ManagerAddressGateway, "https://user-gateway.example.com")
	insertSettingsManagerAddress(t, userManager.ID, codespace_model.ManagerAddressSSH, "ssh.example.com:2222")

	codespaceUUID := "51515151-5151-4151-8151-515151515151"
	insertServiceCodespace(t, userManager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 51,
	})
	require.NoError(t, db.Insert(t.Context(), &codespace_model.GiteaToken{
		CodespaceID:    loadServiceCodespace(t, codespaceUUID).ID,
		TokenHash:      "manager-delete-hash",
		TokenSalt:      "salt",
		TokenLastEight: "last0001",
		TokenEncrypted: "encrypted",
	}))
	require.NoError(t, db.Insert(t.Context(), &codespace_model.SSHKey{
		CodespaceID: loadServiceCodespace(t, codespaceUUID).ID,
		KeyID:       5151,
	}))

	siteSettings, err := ListManagerSettings(t.Context(), ManagerSettingsOptions{Scope: ManagerSettingsScopeSite})
	require.NoError(t, err)
	assert.Len(t, siteSettings.Managers, 2)

	userSettings, err := ListManagerSettings(t.Context(), ManagerSettingsOptions{
		Scope:  ManagerSettingsScopeUser,
		UserID: 1,
	})
	require.NoError(t, err)
	require.Len(t, userSettings.Managers, 1)
	assert.Equal(t, userManager.ID, userSettings.Managers[0].ID)
	assert.EqualValues(t, 1, userSettings.Managers[0].BoundCodespaces)
	assert.Equal(t, "https://user-gateway.example.com", userSettings.Managers[0].GatewayURL)
	assert.Equal(t, "0.2.0", userSettings.Managers[0].Version)
	assert.Equal(t, []ManagerEnvironmentDeclaration{{Tag: "default", Description: "Personal environment"}, {Tag: "gpu"}}, userSettings.Managers[0].Environments)
	assert.Equal(t, "ssh-ed25519", userSettings.Managers[0].GatewaySSHHostKeyAlgorithm)
	assert.Equal(t, "SHA256:settings", userSettings.Managers[0].GatewaySSHHostKeyFingerprintSHA256)
	assert.EqualValues(t, 123, userSettings.Managers[0].GatewaySSHHostKeyUpdatedUnix)
	detail, err := GetManagerDetail(t.Context(), ManagerDetailOptions{
		ManagerSettingsOptions: ManagerSettingsOptions{Scope: ManagerSettingsScopeUser, UserID: 1},
		ManagerID:              userManager.ID,
		Page:                   1,
		PageSize:               30,
	})
	require.NoError(t, err)
	assert.Equal(t, []ManagerEnvironmentDeclaration{{Tag: "default", Description: "Personal environment"}, {Tag: "gpu"}}, detail.Manager.Environments)
	assert.Equal(t, []string{"default"}, detail.Manager.EnvironmentDescriptionConflicts)
	require.Len(t, detail.Codespaces, 1)
	assert.Equal(t, codespaceUUID, detail.Codespaces[0].UUID)
	assert.EqualValues(t, 1, detail.Total)
	_, err = GetManagerDetail(t.Context(), ManagerDetailOptions{
		ManagerSettingsOptions: ManagerSettingsOptions{Scope: ManagerSettingsScopeUser, UserID: 2},
		ManagerID:              userManager.ID,
		Page:                   1,
		PageSize:               30,
	})
	require.ErrorIs(t, err, ErrManagerSettingsNotFound)

	err = DeleteManager(t.Context(), DeleteManagerOptions{
		Scope:     ManagerSettingsScopeUser,
		UserID:    1,
		ManagerID: globalManager.ID,
		Confirm:   true,
	})
	require.ErrorIs(t, err, ErrManagerSettingsNotFound)
	err = DeleteManager(t.Context(), DeleteManagerOptions{
		Scope:     ManagerSettingsScopeUser,
		UserID:    1,
		ManagerID: userManager.ID,
	})
	require.ErrorIs(t, err, ErrManagerSettingsConfirmRequired)

	require.NoError(t, DeleteManager(t.Context(), DeleteManagerOptions{
		Scope:     ManagerSettingsScopeUser,
		UserID:    1,
		ManagerID: userManager.ID,
		Confirm:   true,
	}))
	assertServiceNotExists(t, new(codespace_model.Manager), "id = ?", userManager.ID)
	assertServiceNotExists(t, new(codespace_model.ManagerAddress), "manager_id = ?", userManager.ID)
	assertServiceNotExists(t, new(codespace_model.Codespace), "uuid = ?", codespaceUUID)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_id = (SELECT id FROM codespace WHERE uuid = ?)", codespaceUUID)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_id = (SELECT id FROM codespace WHERE uuid = ?)", codespaceUUID)
}

func TestPersonalManagerDeleteRejectsForeignBindingBeforeCleanup(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	manager.UserID = 1
	_, err := db.GetEngine(t.Context()).ID(manager.ID).Cols("user_id").Update(manager)
	require.NoError(t, err)
	codespaceUUID := "52525252-5252-4252-8252-525252525252"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{UUID: codespaceUUID, Status: codespace_model.StatusStopped})
	_, err = db.GetEngine(t.Context()).Where("uuid = ?", codespaceUUID).Cols("user_id").Update(&codespace_model.Codespace{UserID: 2})
	require.NoError(t, err)

	err = DeleteManager(t.Context(), DeleteManagerOptions{
		Scope: ManagerSettingsScopeUser, UserID: 1, ManagerID: manager.ID, Confirm: true,
	})
	require.ErrorIs(t, err, ErrManagerSettingsOwnershipConflict)
	assertServiceExists(t, new(codespace_model.Manager), "id = ?", manager.ID)
	assertServiceExists(t, new(codespace_model.Codespace), "uuid = ?", codespaceUUID)
}

func TestManagerSettingsRequireIndividualUser(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	_, err := CreateManager(t.Context(), ManagerSettingsOptions{
		Scope:  ManagerSettingsScopeUser,
		UserID: 3,
	})
	require.ErrorContains(t, err, "not an individual")
}

func insertSettingsManagerAddress(t *testing.T, managerID int64, kind, address string) {
	t.Helper()
	require.NoError(t, db.Insert(t.Context(), &codespace_model.ManagerAddress{
		ManagerID: managerID,
		Kind:      kind,
		Address:   address,
	}))
}
