// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"path/filepath"
	"strings"
	"testing"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	asymkey_model "gitea.dev/models/asymkey"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/models/unit"
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

func requestRuntimeGitSSHKey(ctx context.Context, manager *codespace_model.Manager, opts RequestRuntimeAccessOptions) ([]string, error) {
	result, err := RequestRuntimeAccess(ctx, manager, opts)
	if err != nil {
		return nil, err
	}
	return result.GitSSHKnownHosts, nil
}

func TestRuntimeGitSSHKeyCreateReturnsStableKnownHosts(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureServiceGitSSHHostKey(t)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1019)
	insertActiveCreateCodespaceForGitSSHKey(t, manager.ID, codespaceID, 15)

	result, err := requestRuntimeGitSSHKey(t.Context(), manager, RequestRuntimeAccessOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 15,
		GitSSHPublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.True(t, strings.HasPrefix(result[0], "[gitea.example.com]:2222 ssh-ed25519 "))
	relation := loadServiceSSHKeyRelation(t, codespaceID)
	publicKey := loadServicePublicKey(t, relation.KeyID)
	assert.EqualValues(t, asymkey_model.KeyTypeCodespace, publicKey.Type)
	assert.False(t, publicKey.Verified)
	assert.Equal(t, serviceCanonicalPublicKey(t, testGitSSHPublicKey), publicKey.Content)

	again, err := requestRuntimeGitSSHKey(t.Context(), manager, RequestRuntimeAccessOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 15,
		GitSSHPublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	assert.Equal(t, result, again)
	assert.Equal(t, relation.KeyID, loadServiceSSHKeyRelation(t, codespaceID).KeyID)
}

func TestNormalizeGitSSHPublicKeyAcceptsRSA4096(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	require.NoError(t, err)
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)

	normalized, err := normalizeGitSSHPublicKey(publicKey.Marshal())
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(normalized.Content, "ssh-rsa "))
}

func TestNormalizeGitSSHPublicKeyRejectsRSA2048(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)

	_, err = normalizeGitSSHPublicKey(publicKey.Marshal())
	require.ErrorIs(t, err, ErrRuntimeGitSSHKeyInvalidPublicKey)
}

func TestResolveGitSSHKeyUserUsesCodespaceBinding(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureServiceGitSSHHostKey(t)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1069)
	insertActiveCreateCodespaceForGitSSHKey(t, manager.ID, codespaceID, 20)
	_, err := requestRuntimeGitSSHKey(t.Context(), manager, RequestRuntimeAccessOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 20,
		GitSSHPublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	_, err = db.GetEngine(t.Context()).Where("id = ?", codespaceID).Cols(
		"status",
		"operation_trigger",
		"operation_created_unix",
		"operation_started_unix",
		"operation_deadline_unix",
	).Update(&codespace_model.Codespace{Status: codespace_model.StatusRunning})
	require.NoError(t, err)
	publicKey := loadServicePublicKey(t, loadServiceSSHKeyRelation(t, codespaceID).KeyID)

	user, err := ResolveGitSSHKeyUser(t.Context(), publicKey, 2, unit.TypeCode, perm.AccessModeRead)
	require.NoError(t, err)
	assert.EqualValues(t, 1, user.ID)

	_, err = ResolveGitSSHKeyUser(t.Context(), publicKey, 3, unit.TypeCode, perm.AccessModeRead)
	require.ErrorIs(t, err, ErrResolveGitSSHKeyRepoMismatch)

	now := time.Now().Unix()
	authorization := &codespace_model.PermissionAuthorization{
		UserID: 1, SourceRepoID: 2, RequestHash: "git-ssh-permissions",
		CreatedUnix: now, UpdatedUnix: now,
	}
	require.NoError(t, db.Insert(t.Context(), authorization))
	rule := &codespace_model.PermissionRepository{
		AuthorizationID: authorization.ID, TargetRepoID: 3, UnitType: unit.TypeCode,
		RequestedMode: perm.AccessModeWrite, GrantedMode: perm.AccessModeRead,
	}
	require.NoError(t, db.Insert(t.Context(), rule))
	updated, err := db.GetEngine(t.Context()).Where("id = ?", codespaceID).Cols("permission_authorization_id").Update(&codespace_model.Codespace{PermissionAuthorizationID: authorization.ID})
	require.NoError(t, err)
	require.EqualValues(t, 1, updated)

	_, err = ResolveGitSSHKeyUser(t.Context(), publicKey, 3, unit.TypeCode, perm.AccessModeRead)
	require.NoError(t, err)
	_, err = ResolveGitSSHKeyUser(t.Context(), publicKey, 3, unit.TypeCode, perm.AccessModeWrite)
	require.ErrorIs(t, err, ErrResolveGitSSHKeyRepoMismatch)

	rule.GrantedMode = perm.AccessModeWrite
	updated, err = db.GetEngine(t.Context()).
		Where("authorization_id = ? AND target_repo_id = ? AND unit_type = ?", rule.AuthorizationID, rule.TargetRepoID, rule.UnitType).
		Cols("granted_mode").Update(rule)
	require.NoError(t, err)
	require.EqualValues(t, 1, updated)
	_, err = ResolveGitSSHKeyUser(t.Context(), publicKey, 3, unit.TypeCode, perm.AccessModeWrite)
	require.NoError(t, err)

	require.NoError(t, RevokePermissionAuthorization(t.Context(), 1, authorization.ID))
	_, err = ResolveGitSSHKeyUser(t.Context(), publicKey, 3, unit.TypeCode, perm.AccessModeRead)
	require.ErrorIs(t, err, ErrResolveGitSSHKeyRepoMismatch)
}

func TestRuntimeGitSSHKeyRejectsDifferentExistingKey(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureServiceGitSSHHostKey(t)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1070)
	insertActiveCreateCodespaceForGitSSHKey(t, manager.ID, codespaceID, 16)
	_, err := requestRuntimeGitSSHKey(t.Context(), manager, RequestRuntimeAccessOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 16,
		GitSSHPublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	original := loadServiceSSHKeyRelation(t, codespaceID)

	_, err = requestRuntimeGitSSHKey(t.Context(), manager, RequestRuntimeAccessOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 16,
		GitSSHPublicKey:   servicePublicKeyWire(t, testOtherGitSSHPublicKey),
	})
	require.ErrorIs(t, err, ErrRuntimeGitSSHKeyConflict)
	assert.Equal(t, original.KeyID, loadServiceSSHKeyRelation(t, codespaceID).KeyID)
}

func TestRuntimeGitSSHKeyRepairsOrphanedSameCodespaceKey(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureServiceGitSSHHostKey(t)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1071)
	insertActiveCreateCodespaceForGitSSHKey(t, manager.ID, codespaceID, 16)
	key, err := normalizeGitSSHPublicKey(servicePublicKeyWire(t, testGitSSHPublicKey))
	require.NoError(t, err)
	publicKey := &asymkey_model.PublicKey{
		OwnerID:     1,
		Name:        codespaceGitSSHKeyName(codespaceID),
		Fingerprint: key.Fingerprint,
		Content:     key.Content,
		Mode:        perm.AccessModeWrite,
		Type:        asymkey_model.KeyTypeCodespace,
	}
	require.NoError(t, db.Insert(t.Context(), publicKey))

	_, err = requestRuntimeGitSSHKey(t.Context(), manager, RequestRuntimeAccessOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 16,
		GitSSHPublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	assert.Equal(t, publicKey.ID, loadServiceSSHKeyRelation(t, codespaceID).KeyID)
}

func TestRuntimeGitSSHKeyAllowsStableRunningRecovery(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureServiceGitSSHHostKey(t)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1072)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 17,
	})

	_, err := requestRuntimeGitSSHKey(t.Context(), manager, RequestRuntimeAccessOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 17,
		GitSSHPublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	assert.NotZero(t, loadServiceSSHKeyRelation(t, codespaceID).KeyID)
}

func TestRuntimeGitSSHKeyUsesCurrentManagerAvailability(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureServiceGitSSHHostKey(t)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	staleManager := *manager
	codespaceID := int64(1073)
	insertActiveCreateCodespaceForGitSSHKey(t, manager.ID, codespaceID, 18)
	_, err := db.GetEngine(t.Context()).
		ID(manager.ID).
		Cols("last_online_unix").
		Update(&codespace_model.Manager{LastOnlineUnix: 1})
	require.NoError(t, err)

	_, err = requestRuntimeGitSSHKey(t.Context(), &staleManager, RequestRuntimeAccessOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 18,
		GitSSHPublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.ErrorIs(t, err, ErrRequestRuntimeAccessManagerOffline)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_id = ?", codespaceID)
}

func TestRuntimeGitSSHKeyRejectsLoginRestrictedUser(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureServiceGitSSHHostKey(t)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1074)
	insertActiveCreateCodespaceForGitSSHKey(t, manager.ID, codespaceID, 19)
	_, err := db.GetEngine(t.Context()).
		ID(1).
		Cols("must_change_password").
		Update(&user_model.User{MustChangePassword: true})
	require.NoError(t, err)

	_, err = requestRuntimeGitSSHKey(t.Context(), manager, RequestRuntimeAccessOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 19,
		GitSSHPublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.ErrorIs(t, err, ErrRuntimeGitSSHKeyLoginRestricted)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_id = ?", codespaceID)
}

func TestRuntimeGitSSHKeyRejectsInvalidPublicKey(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureServiceGitSSHHostKey(t)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1075)
	insertActiveCreateCodespaceForGitSSHKey(t, manager.ID, codespaceID, 1)
	_, err := requestRuntimeGitSSHKey(t.Context(), manager, RequestRuntimeAccessOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 1,
		GitSSHPublicKey:   []byte("not-ssh-wire"),
	})
	require.ErrorIs(t, err, ErrRuntimeGitSSHKeyInvalidPublicKey)
}

func TestRuntimeGitSSHKeyAllowsHTTPCloneWithoutKnownHosts(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureGitTransportTestSettings(t, codespace_model.GitProtocolHTTP, false, false, nil)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1069)
	insertActiveCreateCodespaceForGitSSHKey(t, manager.ID, codespaceID, 1)

	result, err := requestRuntimeGitSSHKey(t.Context(), manager, RequestRuntimeAccessOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 1,
		GitSSHPublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	assert.Empty(t, result)
	assert.NotZero(t, loadServiceSSHKeyRelation(t, codespaceID).KeyID)
}

func TestRuntimeGitSSHKeyReturnsConfiguredSSHKnownHostsForHTTPClone(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureGitTransportTestSettings(t, codespace_model.GitProtocolHTTP, false, false, []string{
		"gitea.example.com " + testGitSSHPublicKey,
	})

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1076)
	insertActiveCreateCodespaceForGitSSHKey(t, manager.ID, codespaceID, 1)

	result, err := requestRuntimeGitSSHKey(t.Context(), manager, RequestRuntimeAccessOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 1,
		GitSSHPublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"gitea.example.com " + testGitSSHPublicKey}, result)
}

func TestRuntimeGitSSHKeyRejectsSSHCloneWithoutKnownHosts(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureGitTransportTestSettings(t, codespace_model.GitProtocolSSH, false, false, nil)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1077)
	insertActiveCreateCodespaceForGitSSHKey(t, manager.ID, codespaceID, 1)

	_, err := requestRuntimeGitSSHKey(t.Context(), manager, RequestRuntimeAccessOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 1,
		GitSSHPublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.ErrorIs(t, err, ErrRequestRuntimeAccessStateUnavailable)
	assert.ErrorContains(t, err, "known hosts are required")
}

func TestFinalizeFailedDeletesCodespaceGitPublicKey(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureServiceGitSSHHostKey(t)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1034)
	insertActiveCreateCodespaceForGitSSHKey(t, manager.ID, codespaceID, 18)
	_, err := requestRuntimeGitSSHKey(t.Context(), manager, RequestRuntimeAccessOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 18,
		GitSSHPublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	keyID := loadServiceSSHKeyRelation(t, codespaceID).KeyID

	outcome, err := FinalizeOperation(t.Context(), manager, FinalizeOperationOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 18,
		FinalStatus:       codespacev1.FinalStatus_FINAL_STATUS_FAILED,
	})
	require.NoError(t, err)
	assert.False(t, outcome.GetResourceAbsent())
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_id = ?", codespaceID)
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

func configureServiceGitSSHHostKey(t *testing.T) {
	t.Helper()
	keyPath := filepath.Join(t.TempDir(), "gitea.ed25519")
	require.NoError(t, ssh_module.GenKeyPair(keyPath, generate.SSHKeyED25519, 0))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Domain, "gitea.example.com"))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Port, 2222))
	t.Cleanup(test.MockVariableValue(&setting.SSH.ServerHostKeys, []string{keyPath}))
	t.Cleanup(test.MockVariableValue(&setting.SSH.StartBuiltinServer, true))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GitSSHKnownHosts, []string(nil)))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GitProtocol, codespace_model.GitProtocolSSH))
}

func insertActiveCreateCodespaceForGitSSHKey(t *testing.T, managerID, codespaceID, operationRVersion int64) {
	t.Helper()
	insertServiceCodespace(t, managerID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     operationRVersion,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
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

func loadServiceSSHKeyRelation(t *testing.T, codespaceID int64) *codespace_model.SSHKey {
	t.Helper()
	row := new(codespace_model.SSHKey)
	has, err := db.GetEngine(t.Context()).ID(loadServiceCodespace(t, codespaceID).ID).Get(row)
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
