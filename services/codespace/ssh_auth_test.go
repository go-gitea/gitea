// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"
	"time"

	asymkey_model "gitea.dev/models/asymkey"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestVerifySSHPublicKeyAllowsAndCancelsQueuedIdleStop(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "71717171-7171-4717-8717-717171717171"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     51,
		OperationType:         codespace_model.OperationStop,
		OperationStatus:       codespace_model.OperationStatusQueued,
		OperationTrigger:      codespace_model.OperationTriggerIdle,
		OperationCreatedUnix:  time.Now().Unix(),
		InteractionGeneration: 9,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 51, []map[string]any{})))
	insertServiceUserSSHKey(t, 1, testGitSSHPublicKey)

	result, err := VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceUUID: codespaceUUID,
		PublicKey:     servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	require.True(t, result.Allowed)
	assert.EqualValues(t, 1, result.UserID)
	assert.EqualValues(t, 10, result.InteractionGeneration)

	row := loadServiceCodespace(t, codespaceUUID)
	assert.EqualValues(t, 10, row.InteractionGeneration)
	assert.Empty(t, row.OperationType)
	assert.Empty(t, row.OperationStatus)
	assert.Empty(t, row.OperationTrigger)
	assert.Positive(t, row.LastActiveUnix)
}

func TestVerifySSHPublicKeyKeepsLifecycleUpdatedUnixWithoutIdleStop(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "71717171-7171-4717-8717-717171717172"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     51,
		InteractionGeneration: 9,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 51, []map[string]any{})))
	insertServiceUserSSHKey(t, 1, testGitSSHPublicKey)
	updatedUnix := loadServiceCodespace(t, codespaceUUID).UpdatedUnix

	result, err := VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceUUID: codespaceUUID,
		PublicKey:     servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	require.True(t, result.Allowed)
	assert.EqualValues(t, 10, result.InteractionGeneration)

	row := loadServiceCodespace(t, codespaceUUID)
	assert.EqualValues(t, 10, row.InteractionGeneration)
	assert.Equal(t, updatedUnix, row.UpdatedUnix)
	assert.Positive(t, row.LastActiveUnix)
}

func TestVerifySSHPublicKeyDeniesInvalidCredentials(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "72727272-7272-4727-8727-727272727272"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     52,
		InteractionGeneration: 3,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 52, []map[string]any{})))
	insertServiceUserSSHKey(t, 1, testGitSSHPublicKey)

	result, err := VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceUUID: codespaceUUID,
		PublicKey:     servicePublicKeyWire(t, testOtherGitSSHPublicKey),
	})
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, SSHAuthDeniedInvalidCredentials, result.DeniedCategory)
	assert.EqualValues(t, 3, loadServiceCodespace(t, codespaceUUID).InteractionGeneration)

	result, err = VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceUUID: codespaceUUID,
		PublicKey:     []byte("not-ssh-wire"),
	})
	require.NoError(t, err)
	assert.Equal(t, SSHAuthDeniedInvalidCredentials, result.DeniedCategory)
}

func TestVerifySSHPublicKeyDeniesStateAndMetadata(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	insertServiceUserSSHKey(t, 1, testGitSSHPublicKey)

	activeUUID := "73737373-7373-4737-8737-737373737373"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 activeUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    53,
		OperationType:        codespace_model.OperationStop,
		OperationStatus:      codespace_model.OperationStatusRunning,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})
	require.NoError(t, putRuntimeMetadataEntry(activeUUID, serviceRuntimeMetadataEntry(t, 53, []map[string]any{})))
	result, err := VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceUUID: activeUUID,
		PublicKey:     servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	assert.Equal(t, SSHAuthDeniedStateUnavailable, result.DeniedCategory)

	missingMetadataUUID := "74747474-7474-4747-8747-747474747474"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              missingMetadataUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 54,
	})
	result, err = VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceUUID: missingMetadataUUID,
		PublicKey:     servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	assert.Equal(t, SSHAuthDeniedMetadataRebuilding, result.DeniedCategory)
}

func TestVerifySSHPublicKeyUsesUnifiedLoginRestrictions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.TwoFactorAuthEnforced, true))

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	insertServiceUserSSHKey(t, 1, testGitSSHPublicKey)
	restrictedUUID := "75757575-7575-4757-8757-757575757575"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  restrictedUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     55,
		InteractionGeneration: 4,
	})
	require.NoError(t, putRuntimeMetadataEntry(restrictedUUID, serviceRuntimeMetadataEntry(t, 55, []map[string]any{})))
	_, err := db.GetEngine(t.Context()).
		ID(1).
		Cols("must_change_password").
		Update(&user_model.User{MustChangePassword: true})
	require.NoError(t, err)

	result, err := VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceUUID: restrictedUUID,
		PublicKey:     servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	assert.Equal(t, SSHAuthDeniedLoginRestricted, result.DeniedCategory)
	assert.EqualValues(t, 4, loadServiceCodespace(t, restrictedUUID).InteractionGeneration)

	insertServiceUserSSHKey(t, 32, testOtherGitSSHPublicKey)
	webauthnUUID := "76767676-7676-4767-8767-767676767676"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  webauthnUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     56,
		InteractionGeneration: 7,
	})
	_, err = db.GetEngine(t.Context()).
		ID(webauthnUUID).
		Cols("user_id").
		Update(&codespace_model.Codespace{UserID: 32})
	require.NoError(t, err)
	require.NoError(t, putRuntimeMetadataEntry(webauthnUUID, serviceRuntimeMetadataEntry(t, 56, []map[string]any{})))

	result, err = VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceUUID: webauthnUUID,
		PublicKey:     servicePublicKeyWire(t, testOtherGitSSHPublicKey),
	})
	require.NoError(t, err)
	require.True(t, result.Allowed)
	assert.EqualValues(t, 32, result.UserID)
	assert.EqualValues(t, 8, result.InteractionGeneration)
}

func insertServiceUserSSHKey(t *testing.T, ownerID int64, content string) {
	t.Helper()
	canonical := serviceCanonicalPublicKey(t, content)
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(canonical))
	require.NoError(t, err)
	require.NoError(t, db.Insert(t.Context(), &asymkey_model.PublicKey{
		OwnerID:     ownerID,
		Name:        "test-user-key",
		Fingerprint: ssh.FingerprintSHA256(publicKey),
		Content:     canonical,
		Mode:        perm.AccessModeWrite,
		Type:        asymkey_model.KeyTypeUser,
		Verified:    true,
	}))
}
