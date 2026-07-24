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

func TestRegistrationTokenSettingsLifecycle(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	opts := ManagerSettingsOptions{Scope: ManagerSettingsScopeUser, OwnerID: 1}
	settings, err := ListManagerSettings(t.Context(), opts)
	require.NoError(t, err)
	require.Len(t, settings.RegistrationToken, 64)

	token, err := GetOrCreateRegistrationToken(t.Context(), opts)
	require.NoError(t, err)
	assert.Equal(t, settings.RegistrationToken, token)

	sameToken, err := GetOrCreateRegistrationToken(t.Context(), opts)
	require.NoError(t, err)
	assert.Equal(t, token, sameToken)

	resetToken, err := ResetRegistrationToken(t.Context(), opts)
	require.NoError(t, err)
	require.Len(t, resetToken, 64)
	assert.NotEqual(t, token, resetToken)

	assertServiceNotExists(t, new(codespace_model.ManagerToken), "token = ?", token)
	assertServiceExists(t, new(codespace_model.ManagerToken), "owner_id = ? AND token = ?", 1, resetToken)
}

func TestRegisterManagerUsesCurrentTokenAndKeepsSecretAfterReset(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	opts := ManagerSettingsOptions{Scope: ManagerSettingsScopeSite}
	token, err := GetOrCreateRegistrationToken(t.Context(), opts)
	require.NoError(t, err)

	manager, secret, err := RegisterManager(t.Context(), token)
	require.NoError(t, err)
	assert.EqualValues(t, 0, manager.OwnerID)
	_, err = AuthenticateManager(t.Context(), manager.ID, secret)
	require.NoError(t, err)

	resetToken, err := ResetRegistrationToken(t.Context(), opts)
	require.NoError(t, err)
	_, err = AuthenticateManager(t.Context(), manager.ID, secret)
	require.NoError(t, err)
	_, _, err = RegisterManager(t.Context(), token)
	require.Error(t, err)

	resetManager, _, err := RegisterManager(t.Context(), resetToken)
	require.NoError(t, err)
	assert.NotEqual(t, manager.ID, resetManager.ID)
}

func TestListManagerSettingsScopesAndDeleteManager(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	orgToken, err := GetOrCreateRegistrationToken(t.Context(), ManagerSettingsOptions{
		Scope:   ManagerSettingsScopeOrganization,
		OwnerID: 3,
	})
	require.NoError(t, err)
	globalManager := insertServiceManager(t)
	orgManager := insertServiceManager(t)
	orgManager.OwnerID = 3
	orgManager.Name = "org-manager"
	orgManager.MetaJSON = `{"version":"0.2.0","gateway_ssh_host_key_algorithm":"ssh-ed25519","gateway_ssh_host_key_fingerprint_sha256":"SHA256:settings","gateway_ssh_host_key_updated_unix":123}`
	_, err = db.GetEngine(t.Context()).ID(orgManager.ID).Cols("owner_id", "name", "meta_json").Update(orgManager)
	require.NoError(t, err)
	insertSettingsManagerAddress(t, globalManager.ID, codespace_model.ManagerAddressGateway, "https://global-gateway.example.com")
	insertSettingsManagerAddress(t, orgManager.ID, codespace_model.ManagerAddressGateway, "https://org-gateway.example.com")
	insertSettingsManagerAddress(t, orgManager.ID, codespace_model.ManagerAddressSSH, "ssh.example.com:2222")

	codespaceUUID := "51515151-5151-4151-8151-515151515151"
	insertServiceCodespace(t, orgManager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 51,
	})
	require.NoError(t, db.Insert(t.Context(), &codespace_model.GiteaToken{
		CodespaceUUID:  codespaceUUID,
		TokenHash:      "manager-delete-hash",
		TokenSalt:      "salt",
		TokenLastEight: "last0001",
		TokenEncrypted: "encrypted",
	}))
	require.NoError(t, db.Insert(t.Context(), &codespace_model.SSHKey{
		CodespaceUUID: codespaceUUID,
		KeyID:         5151,
		CreatedUnix:   1,
	}))

	siteSettings, err := ListManagerSettings(t.Context(), ManagerSettingsOptions{Scope: ManagerSettingsScopeSite})
	require.NoError(t, err)
	assert.Len(t, siteSettings.Managers, 2)

	orgSettings, err := ListManagerSettings(t.Context(), ManagerSettingsOptions{
		Scope:   ManagerSettingsScopeOrganization,
		OwnerID: 3,
	})
	require.NoError(t, err)
	require.Len(t, orgSettings.Managers, 1)
	assert.Equal(t, orgManager.ID, orgSettings.Managers[0].ID)
	assert.EqualValues(t, 1, orgSettings.Managers[0].BoundCodespaces)
	assert.Equal(t, "https://org-gateway.example.com", orgSettings.Managers[0].GatewayURL)
	assert.Equal(t, "0.2.0", orgSettings.Managers[0].Version)
	assert.Equal(t, "ssh-ed25519", orgSettings.Managers[0].GatewaySSHHostKeyAlgorithm)
	assert.Equal(t, "SHA256:settings", orgSettings.Managers[0].GatewaySSHHostKeyFingerprintSHA256)
	assert.EqualValues(t, 123, orgSettings.Managers[0].GatewaySSHHostKeyUpdatedUnix)

	err = DeleteManager(t.Context(), DeleteManagerOptions{
		Scope:     ManagerSettingsScopeOrganization,
		OwnerID:   3,
		ManagerID: globalManager.ID,
		Confirm:   true,
	})
	require.ErrorIs(t, err, ErrManagerSettingsNotFound)
	err = DeleteManager(t.Context(), DeleteManagerOptions{
		Scope:     ManagerSettingsScopeOrganization,
		OwnerID:   3,
		ManagerID: orgManager.ID,
	})
	require.ErrorIs(t, err, ErrManagerSettingsConfirmRequired)

	require.NoError(t, DeleteManager(t.Context(), DeleteManagerOptions{
		Scope:     ManagerSettingsScopeOrganization,
		OwnerID:   3,
		ManagerID: orgManager.ID,
		Confirm:   true,
	}))
	assertServiceNotExists(t, new(codespace_model.Manager), "id = ?", orgManager.ID)
	assertServiceNotExists(t, new(codespace_model.ManagerAddress), "manager_id = ?", orgManager.ID)
	assertServiceNotExists(t, new(codespace_model.Codespace), "uuid = ?", codespaceUUID)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", codespaceUUID)
	assertServiceExists(t, new(codespace_model.ManagerToken), "owner_id = ? AND token = ?", 3, orgToken)
}

func insertSettingsManagerAddress(t *testing.T, managerID int64, kind, address string) {
	t.Helper()
	require.NoError(t, db.Insert(t.Context(), &codespace_model.ManagerAddress{
		ManagerID: managerID,
		Kind:      kind,
		Address:   address,
	}))
}
