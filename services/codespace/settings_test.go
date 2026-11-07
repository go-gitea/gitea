// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"fmt"
	"sync"
	"testing"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagerRegistrationTokenLifecycle(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	opts := ManagerSettingsOptions{Scope: ManagerSettingsScopeUser, UserID: 1}
	settings, err := ListManagerSettings(t.Context(), opts)
	require.NoError(t, err)
	require.Len(t, settings.RegistrationToken, 40)

	stable, err := ListManagerSettings(t.Context(), opts)
	require.NoError(t, err)
	assert.Equal(t, settings.RegistrationToken, stable.RegistrationToken)

	manager, secret, err := RegisterManager(t.Context(), settings.RegistrationToken, " Personal Manager ")
	require.NoError(t, err)
	assert.Equal(t, "Personal Manager", manager.Name)
	assert.EqualValues(t, 1, manager.UserID)
	assert.Equal(t, codespace_model.ManagerRuntimeStateRecovering, manager.RuntimeState)
	authenticated, err := AuthenticateManager(t.Context(), manager.ID, secret)
	require.NoError(t, err)
	assert.Equal(t, manager.ID, authenticated.ID)

	second, secondSecret, err := RegisterManager(t.Context(), settings.RegistrationToken, "Second Manager")
	require.NoError(t, err)
	assert.NotEqual(t, manager.ID, second.ID)
	assert.NotEqual(t, secret, secondSecret)

	rotated, err := RotateManagerRegistrationToken(t.Context(), opts)
	require.NoError(t, err)
	assert.NotEqual(t, settings.RegistrationToken, rotated)
	_, _, err = RegisterManager(t.Context(), settings.RegistrationToken, "Old Token")
	require.ErrorIs(t, err, ErrManagerRegistrationTokenInvalid)
	third, _, err := RegisterManager(t.Context(), rotated, "Third Manager")
	require.NoError(t, err)
	assert.EqualValues(t, 1, third.UserID)

	_, err = AuthenticateManager(t.Context(), manager.ID, secret)
	require.NoError(t, err)
	require.NoError(t, UpdateManagerName(t.Context(), opts, manager.ID, " Renamed "))
	authenticated, err = AuthenticateManager(t.Context(), manager.ID, secret)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", authenticated.Name)

	otherUser := ManagerSettingsOptions{Scope: ManagerSettingsScopeUser, UserID: 2}
	require.ErrorIs(t, UpdateManagerName(t.Context(), otherUser, manager.ID, "Other"), ErrManagerSettingsNotFound)
}

func TestRegisterManagerRejectsInvalidName(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	settings, err := ListManagerSettings(t.Context(), ManagerSettingsOptions{Scope: ManagerSettingsScopeSite})
	require.NoError(t, err)

	for _, name := range []string{"", " \t ", "bad\nname"} {
		_, _, err := RegisterManager(t.Context(), settings.RegistrationToken, name)
		require.ErrorIs(t, err, ErrManagerSettingsNameInvalid)
	}
}

func TestRegisterManagerAllowsConcurrentUseOfScopeToken(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	settings, err := ListManagerSettings(t.Context(), ManagerSettingsOptions{Scope: ManagerSettingsScopeSite})
	require.NoError(t, err)

	const registrations = 4
	managerIDs := make(chan int64, registrations)
	errors := make(chan error, registrations)
	var wait sync.WaitGroup
	for i := range registrations {
		wait.Go(func() {
			manager, _, err := RegisterManager(t.Context(), settings.RegistrationToken, fmt.Sprintf("Manager %d", i+1))
			if err != nil {
				errors <- err
				return
			}
			managerIDs <- manager.ID
		})
	}
	wait.Wait()
	close(errors)
	close(managerIDs)
	for err := range errors {
		require.NoError(t, err)
	}
	unique := make(map[int64]struct{}, registrations)
	for managerID := range managerIDs {
		unique[managerID] = struct{}{}
	}
	assert.Len(t, unique, registrations)
}

func TestListManagerSettingsScopesAndDeleteManager(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	globalManager := insertServiceManager(t)
	globalManager.TagsJSON = `[{"tag":"default","description":"Site environment"}]`
	globalManager.GatewayURL = "https://global-gateway.example.com"
	_, err := db.GetEngine(t.Context()).ID(globalManager.ID).Cols("tags_json", "gateway_url").Update(globalManager)
	require.NoError(t, err)
	userManager := insertServiceManager(t)
	userManager.UserID = 1
	userManager.Name = "user-manager"
	userManager.Version = "0.2.0"
	userManager.TagsJSON = `[{"tag":"default","description":"Personal environment"},{"tag":"gpu"}]`
	userManager.GatewayURL = "https://user-gateway.example.com"
	userManager.GatewaySSHAddr = "ssh.example.com:2222"
	userManager.GatewaySSHHostKeyAlgorithm = "ssh-ed25519"
	userManager.GatewaySSHHostKeyFingerprintSHA256 = "SHA256:settings"
	_, err = db.GetEngine(t.Context()).ID(userManager.ID).Cols(
		"user_id", "name", "version", "tags_json", "gateway_url", "gateway_ssh_addr",
		"gateway_ssh_host_key_algorithm", "gateway_ssh_host_key_fingerprint_sha256",
	).Update(userManager)
	require.NoError(t, err)

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

	unbound := &codespace_model.Codespace{ManagerID: userManager.ID, UserID: 1, Status: codespace_model.StatusCreating}
	_, err = db.GetEngine(t.Context()).Table(unbound).Omit("uuid").Insert(unbound)
	require.NoError(t, err)
	require.NoError(t, DeleteManager(t.Context(), DeleteManagerOptions{
		Scope:     ManagerSettingsScopeUser,
		UserID:    1,
		ManagerID: userManager.ID,
		Confirm:   true,
	}))
	assertServiceNotExists(t, new(codespace_model.Manager), "id = ?", userManager.ID)
	assertServiceNotExists(t, new(codespace_model.Codespace), "id = ?", unbound.ID)
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

	_, err := ListManagerSettings(t.Context(), ManagerSettingsOptions{Scope: ManagerSettingsScopeUser, UserID: 3})
	require.ErrorContains(t, err, "not an individual")
}
