// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	asymkey_model "gitea.dev/models/asymkey"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/generate"
	"gitea.dev/modules/setting"
	ssh_module "gitea.dev/modules/ssh"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

const (
	testGitSSHPublicKey      = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICV0MGX/W9IvLA4FXpIuUcdDcbj5KX4syHgsTy7soVgf"
	testOtherGitSSHPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEHjnNEfE88W1pvBLdV3otv28x760gdmPao3lVD5uAt9"
)

func TestEnsureGitSSHKeyCreateReturnsStableKnownHosts(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureServiceGitSSHHostKey(t)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "12121212-1212-4212-8212-121212121212"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     15,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
		GitProtocol:           codespace_model.GitProtocolHTTP,
	})

	result, err := EnsureGitSSHKey(t.Context(), manager, EnsureGitSSHKeyOptions{
		CodespaceUUID: codespaceUUID,
		PublicKey:     servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	require.Len(t, result.KnownHostsLines, 1)
	assert.True(t, strings.HasPrefix(result.KnownHostsLines[0], "[gitea.example.com]:2222 ssh-ed25519 "))
	relation := loadServiceSSHKeyRelation(t, codespaceUUID)
	publicKey := loadServicePublicKey(t, relation.KeyID)
	assert.EqualValues(t, asymkey_model.KeyTypeCodespace, publicKey.Type)
	assert.False(t, publicKey.Verified)
	assert.Equal(t, serviceCanonicalPublicKey(t, testGitSSHPublicKey), publicKey.Content)

	again, err := EnsureGitSSHKey(t.Context(), manager, EnsureGitSSHKeyOptions{
		CodespaceUUID: codespaceUUID,
		PublicKey:     servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	assert.Equal(t, result.KnownHostsLines, again.KnownHostsLines)
	assert.Equal(t, relation.KeyID, loadServiceSSHKeyRelation(t, codespaceUUID).KeyID)
}

func TestResolveGitSSHKeyUserUsesCodespaceBinding(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureServiceGitSSHHostKey(t)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "13131313-1313-4313-8313-131313131313"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     20,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	_, err := EnsureGitSSHKey(t.Context(), manager, EnsureGitSSHKeyOptions{
		CodespaceUUID: codespaceUUID,
		PublicKey:     servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	_, err = db.GetEngine(t.Context()).ID(codespaceUUID).Cols(
		"status",
		"operation_type",
		"operation_status",
		"operation_trigger",
	).Update(&codespace_model.Codespace{Status: codespace_model.StatusRunning})
	require.NoError(t, err)
	publicKey := loadServicePublicKey(t, loadServiceSSHKeyRelation(t, codespaceUUID).KeyID)

	user, err := ResolveGitSSHKeyUser(t.Context(), publicKey, 2)
	require.NoError(t, err)
	assert.EqualValues(t, 1, user.ID)

	_, err = ResolveGitSSHKeyUser(t.Context(), publicKey, 3)
	require.ErrorIs(t, err, ErrResolveGitSSHKeyRepoMismatch)
}

func TestEnsureGitSSHKeyRejectsDifferentExistingKey(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureServiceGitSSHHostKey(t)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "23232323-2323-4232-8232-232323232323"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     16,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	_, err := EnsureGitSSHKey(t.Context(), manager, EnsureGitSSHKeyOptions{
		CodespaceUUID: codespaceUUID,
		PublicKey:     servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	original := loadServiceSSHKeyRelation(t, codespaceUUID)

	_, err = EnsureGitSSHKey(t.Context(), manager, EnsureGitSSHKeyOptions{
		CodespaceUUID: codespaceUUID,
		PublicKey:     servicePublicKeyWire(t, testOtherGitSSHPublicKey),
	})
	require.ErrorIs(t, err, ErrEnsureGitSSHKeyConflict)
	assert.Equal(t, original.KeyID, loadServiceSSHKeyRelation(t, codespaceUUID).KeyID)
}

func TestEnsureGitSSHKeyRejectsUnavailableState(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureServiceGitSSHHostKey(t)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "34343434-3434-4343-8343-343434343434"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 17,
	})

	_, err := EnsureGitSSHKey(t.Context(), manager, EnsureGitSSHKeyOptions{
		CodespaceUUID: codespaceUUID,
		PublicKey:     servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.ErrorIs(t, err, ErrEnsureGitSSHKeyStateUnavailable)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", codespaceUUID)
}

func TestEnsureGitSSHKeyUsesCurrentManagerAvailability(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureServiceGitSSHHostKey(t)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	staleManager := *manager
	codespaceUUID := "45454545-4545-4454-8454-454545454546"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     18,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	_, err := db.GetEngine(t.Context()).
		ID(manager.ID).
		Cols("last_online_unix").
		Update(&codespace_model.Manager{LastOnlineUnix: 1})
	require.NoError(t, err)

	_, err = EnsureGitSSHKey(t.Context(), &staleManager, EnsureGitSSHKeyOptions{
		CodespaceUUID: codespaceUUID,
		PublicKey:     servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.ErrorIs(t, err, ErrEnsureGitSSHKeyManagerOffline)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", codespaceUUID)
}

func TestEnsureGitSSHKeyRejectsLoginRestrictedUser(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureServiceGitSSHHostKey(t)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "46464646-4646-4464-8464-464646464646"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     19,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	_, err := db.GetEngine(t.Context()).
		ID(1).
		Cols("must_change_password").
		Update(&user_model.User{MustChangePassword: true})
	require.NoError(t, err)

	_, err = EnsureGitSSHKey(t.Context(), manager, EnsureGitSSHKeyOptions{
		CodespaceUUID: codespaceUUID,
		PublicKey:     servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.ErrorIs(t, err, ErrEnsureGitSSHKeyLoginRestricted)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", codespaceUUID)
}

func TestEnsureGitSSHKeyRejectsInvalidPublicKey(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureServiceGitSSHHostKey(t)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	_, err := EnsureGitSSHKey(t.Context(), manager, EnsureGitSSHKeyOptions{
		CodespaceUUID: "45454545-4545-4454-8454-454545454545",
		PublicKey:     []byte("not-ssh-wire"),
	})
	require.ErrorIs(t, err, ErrEnsureGitSSHKeyInvalidPublicKey)
}

func TestEnsureGitSSHKeyRejectsDisabledSSHClone(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureGitTransportTestSettings(t, codespace_model.GitProtocolHTTP, false, false, nil)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "13131313-1313-4313-8313-131313131313"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     1,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationStartedUnix:  time.Now().Unix(),
		OperationDeadlineUnix: time.Now().Add(time.Minute).Unix(),
	})

	_, err := EnsureGitSSHKey(t.Context(), manager, EnsureGitSSHKeyOptions{
		CodespaceUUID: codespaceUUID,
		PublicKey:     servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.ErrorIs(t, err, ErrEnsureGitSSHKeyStateUnavailable)
	assert.ErrorContains(t, err, "known hosts are required")
}

func TestFinalizeFailedDeletesCodespaceGitPublicKey(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureServiceGitSSHHostKey(t)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "56565656-5656-4565-8565-565656565656"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     18,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	_, err := EnsureGitSSHKey(t.Context(), manager, EnsureGitSSHKeyOptions{
		CodespaceUUID: codespaceUUID,
		PublicKey:     servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	keyID := loadServiceSSHKeyRelation(t, codespaceUUID).KeyID

	outcome, err := FinalizeOperation(t.Context(), manager, FinalizeOperationOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 18,
		OperationType:     codespace_model.OperationCreate,
		FinalStatus:       FinalStatusFailed,
	})
	require.NoError(t, err)
	assert.Equal(t, FinalizeOutcomeAccepted, outcome)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", codespaceUUID)
	assertServiceNotExists(t, new(asymkey_model.PublicKey), "id = ?", keyID)
}

func TestGitSSHKnownHostsLinesUsesConfiguredKnownHosts(t *testing.T) {
	t.Cleanup(test.MockVariableValue(&setting.SSH.Domain, "gitea.example.com"))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Port, 2222))
	t.Cleanup(test.MockVariableValue(&setting.SSH.StartBuiltinServer, false))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GitSSHKnownHosts, []string{
		"[gitea.example.com]:2222 " + testGitSSHPublicKey + " gitea",
	}))

	lines, err := gitSSHKnownHostsLines()
	require.NoError(t, err)
	assert.Equal(t, []string{"[gitea.example.com]:2222 " + testGitSSHPublicKey + " gitea"}, lines)
}

func TestGitSSHKnownHostsLinesRejectsExternalSSHWithoutConfiguredKnownHosts(t *testing.T) {
	t.Cleanup(test.MockVariableValue(&setting.SSH.Domain, "gitea.example.com"))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Port, 22))
	t.Cleanup(test.MockVariableValue(&setting.SSH.StartBuiltinServer, false))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GitSSHKnownHosts, []string(nil)))

	_, err := gitSSHKnownHostsLines()
	require.ErrorContains(t, err, "known hosts are required")
}

func TestValidateGitTransports(t *testing.T) {
	t.Run("http preferred allows external ssh without known hosts", func(t *testing.T) {
		configureGitTransportTestSettings(t, codespace_model.GitProtocolHTTP, false, false, nil)
		require.NoError(t, ValidateGitTransports())
	})

	t.Run("ssh preferred requires known hosts", func(t *testing.T) {
		configureGitTransportTestSettings(t, codespace_model.GitProtocolSSH, false, false, nil)
		err := ValidateGitTransports()
		require.Error(t, err)
		assert.ErrorContains(t, err, "known hosts are required")
	})

	t.Run("configured external ssh enables ssh preferred", func(t *testing.T) {
		configureGitTransportTestSettings(t, codespace_model.GitProtocolSSH, false, false, []string{
			"gitea.example.com " + testGitSSHPublicKey,
		})
		require.NoError(t, ValidateGitTransports())
	})

	t.Run("ssh disabled rejects ssh preferred", func(t *testing.T) {
		configureGitTransportTestSettings(t, codespace_model.GitProtocolSSH, false, true, nil)
		err := ValidateGitTransports()
		require.Error(t, err)
		assert.ErrorContains(t, err, "[server] DISABLE_SSH=true")
	})

	t.Run("http disabled rejects http preferred", func(t *testing.T) {
		configureGitTransportTestSettings(t, codespace_model.GitProtocolHTTP, true, false, []string{
			"gitea.example.com " + testGitSSHPublicKey,
		})
		err := ValidateGitTransports()
		require.Error(t, err)
		assert.ErrorContains(t, err, "[repository] DISABLE_HTTP_GIT=true")
	})

	t.Run("no clone transport rejects startup", func(t *testing.T) {
		configureGitTransportTestSettings(t, codespace_model.GitProtocolHTTP, true, false, nil)
		err := ValidateGitTransports()
		require.Error(t, err)
		assert.ErrorContains(t, err, "[repository] DISABLE_HTTP_GIT=true")
		assert.ErrorContains(t, err, "known hosts are required")
	})
}

func configureGitTransportTestSettings(t *testing.T, protocol string, disableHTTPGit, disableSSH bool, knownHosts []string) {
	t.Helper()
	t.Cleanup(test.MockVariableValue(&setting.Repository.DisableHTTPGit, disableHTTPGit))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Disabled, disableSSH))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Domain, "gitea.example.com"))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Port, 22))
	t.Cleanup(test.MockVariableValue(&setting.SSH.StartBuiltinServer, false))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GitSSHKnownHosts, knownHosts))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GitProtocol, protocol))
}

func configureServiceGitSSHHostKey(t *testing.T) {
	t.Helper()
	keyPath := filepath.Join(t.TempDir(), "gitea.ed25519")
	require.NoError(t, ssh_module.GenKeyPair(keyPath, generate.SSHKeyED25519, 0))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Domain, "gitea.example.com"))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Port, 2222))
	t.Cleanup(test.MockVariableValue(&setting.SSH.ServerHostKeys, []string{keyPath}))
	t.Cleanup(test.MockVariableValue(&setting.SSH.StartBuiltinServer, true))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GitSSHKnownHosts, []string(nil)))
}

func servicePublicKeyWire(t *testing.T, content string) []byte {
	t.Helper()
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(content))
	require.NoError(t, err)
	return publicKey.Marshal()
}

func serviceCanonicalPublicKey(t *testing.T, content string) string {
	t.Helper()
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(content))
	require.NoError(t, err)
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(publicKey)))
}

func loadServiceSSHKeyRelation(t *testing.T, codespaceUUID string) *codespace_model.SSHKey {
	t.Helper()
	row := new(codespace_model.SSHKey)
	has, err := db.GetEngine(t.Context()).ID(codespaceUUID).Get(row)
	require.NoError(t, err)
	require.True(t, has)
	return row
}

func loadServicePublicKey(t *testing.T, keyID int64) *asymkey_model.PublicKey {
	t.Helper()
	row := new(asymkey_model.PublicKey)
	has, err := db.GetEngine(t.Context()).ID(keyID).Get(row)
	require.NoError(t, err)
	require.True(t, has)
	return row
}
