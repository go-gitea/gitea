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
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1084)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     51,
		OperationTrigger:      codespace_model.OperationTriggerIdle,
		OperationCreatedUnix:  time.Now().Unix(),
		InteractionGeneration: 9,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, serviceRuntimeMetadataEntry(t, 51, nil)))
	insertServiceUserSSHKey(t, 1, testGitSSHPublicKey)

	result, err := VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceID: codespaceID,
		PublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	require.NotNil(t, result.GetAllowed())
	assert.EqualValues(t, 1, result.GetAllowed().GetUserId())
	assert.EqualValues(t, 10, result.GetAllowed().GetInteractionGeneration())

	row := loadServiceCodespace(t, codespaceID)
	assert.EqualValues(t, 10, row.InteractionGeneration)
	assert.False(t, hasActiveOperation(row))
	assert.Empty(t, row.OperationTrigger)
	assert.Positive(t, row.LastActiveUnix)
}

func TestVerifySSHPublicKeyKeepsLifecycleUpdatedUnixWithoutIdleStop(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1111)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     51,
		InteractionGeneration: 9,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, serviceRuntimeMetadataEntry(t, 51, nil)))
	insertServiceUserSSHKey(t, 1, testGitSSHPublicKey)
	updatedUnix := loadServiceCodespace(t, codespaceID).UpdatedUnix

	result, err := VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceID: codespaceID,
		PublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	require.NotNil(t, result.GetAllowed())
	assert.EqualValues(t, 10, result.GetAllowed().GetInteractionGeneration())

	row := loadServiceCodespace(t, codespaceID)
	assert.EqualValues(t, 10, row.InteractionGeneration)
	assert.Equal(t, updatedUnix, row.UpdatedUnix)
	assert.Positive(t, row.LastActiveUnix)
}

func TestVerifySSHPublicKeyDeniesInvalidCredentials(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1087)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     52,
		InteractionGeneration: 3,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, serviceRuntimeMetadataEntry(t, 52, nil)))
	insertServiceUserSSHKey(t, 1, testGitSSHPublicKey)

	result, err := VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceID: codespaceID,
		PublicKey:   servicePublicKeyWire(t, testOtherGitSSHPublicKey),
	})
	require.NoError(t, err)
	assert.Equal(t, SSHAuthDeniedInvalidCredentials, result.GetDenied().GetCategory())
	assert.EqualValues(t, 3, loadServiceCodespace(t, codespaceID).InteractionGeneration)

	result, err = VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceID: codespaceID,
		PublicKey:   []byte("not-ssh-wire"),
	})
	require.NoError(t, err)
	assert.Equal(t, SSHAuthDeniedInvalidCredentials, result.GetDenied().GetCategory())
}

func TestVerifySSHPublicKeyDeniesStateAndMetadata(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	insertServiceUserSSHKey(t, 1, testGitSSHPublicKey)

	activeID := int64(1089)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                   activeID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    53,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})
	require.NoError(t, putRuntimeMetadataEntry(activeID, serviceRuntimeMetadataEntry(t, 53, nil)))
	result, err := VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceID: activeID,
		PublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	assert.Equal(t, SSHAuthDeniedStateUnavailable, result.GetDenied().GetCategory())

	missingMetadataID := int64(1090)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                missingMetadataID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 54,
	})
	result, err = VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceID: missingMetadataID,
		PublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	assert.Equal(t, SSHAuthDeniedMetadataRebuilding, result.GetDenied().GetCategory())
}

func TestVerifySSHPublicKeyUsesUnifiedLoginRestrictions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.TwoFactorAuthEnforced, true))

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	insertServiceUserSSHKey(t, 1, testGitSSHPublicKey)
	restrictedID := int64(1112)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    restrictedID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     55,
		InteractionGeneration: 4,
	})
	require.NoError(t, putRuntimeMetadataEntry(restrictedID, serviceRuntimeMetadataEntry(t, 55, nil)))
	_, err := db.GetEngine(t.Context()).
		ID(1).
		Cols("must_change_password").
		Update(&user_model.User{MustChangePassword: true})
	require.NoError(t, err)

	result, err := VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceID: restrictedID,
		PublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	assert.Equal(t, SSHAuthDeniedLoginRestricted, result.GetDenied().GetCategory())
	assert.EqualValues(t, 4, loadServiceCodespace(t, restrictedID).InteractionGeneration)

	insertServiceUserSSHKey(t, 32, testOtherGitSSHPublicKey)
	webauthnID := int64(1113)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    webauthnID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     56,
		InteractionGeneration: 7,
	})
	_, err = db.GetEngine(t.Context()).
		Where("id = ?", webauthnID).
		Cols("user_id").
		Update(&codespace_model.Codespace{UserID: 32})
	require.NoError(t, err)
	require.NoError(t, putRuntimeMetadataEntry(webauthnID, serviceRuntimeMetadataEntry(t, 56, nil)))

	result, err = VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceID: webauthnID,
		PublicKey:   servicePublicKeyWire(t, testOtherGitSSHPublicKey),
	})
	require.NoError(t, err)
	require.NotNil(t, result.GetAllowed())
	assert.EqualValues(t, 32, result.GetAllowed().GetUserId())
	assert.EqualValues(t, 8, result.GetAllowed().GetInteractionGeneration())
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
