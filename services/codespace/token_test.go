// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"strings"
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/globallock"
	secret_module "gitea.dev/modules/secret"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requestRuntimeCredentials(ctx context.Context, manager *codespace_model.Manager, opts requestRuntimeCredentialsOptions) (*requestRuntimeCredentialsResult, error) {
	if err := validateRuntimeAccessRequest(manager, opts.CodespaceID, opts.OperationRVersion); err != nil {
		return nil, err
	}
	var result *requestRuntimeCredentialsResult
	err := globallock.LockAndDo(ctx, runtimeAccessLockKey(opts.CodespaceID), func(ctx context.Context) error {
		prepared, err := prepareRuntimeAccessLocked(ctx, manager, opts)
		if err == nil {
			result = prepared.credentials
		}
		return err
	})
	return result, err
}

func TestRuntimeCredentialsCreateReturnsStableToken(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1140)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     7,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	secretsEncrypted, err := encryptCreateSecrets(t.Context(), 1, false, []CreateSecretInput{{Name: "RUNTIME_SECRET", Source: SecretSourceCustom, Value: "runtime-value"}})
	require.NoError(t, err)
	_, err = db.GetEngine(t.Context()).Where("id = ?", codespaceID).Cols("secrets_encrypted").Update(&codespace_model.Codespace{SecretsEncrypted: secretsEncrypted})
	require.NoError(t, err)

	first, err := requestRuntimeCredentials(t.Context(), manager, requestRuntimeCredentialsOptions{CodespaceID: codespaceID, OperationRVersion: 7})
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.True(t, strings.HasPrefix(first.Token, codespaceTokenPrefix))
	assert.Len(t, first.Token, len(codespaceTokenPrefix)+64)
	assert.NotEmpty(t, first.ServerURL)
	assert.Equal(t, []RuntimeSecret{{Name: "RUNTIME_SECRET", Value: "runtime-value"}}, first.Secrets)
	row := loadServiceGiteaToken(t, codespaceID)
	assert.Equal(t, first.Token[len(first.Token)-8:], row.TokenLastEight)
	assert.True(t, verifyCodespaceGiteaToken(row, first.Token))

	second, err := requestRuntimeCredentials(t.Context(), manager, requestRuntimeCredentialsOptions{CodespaceID: codespaceID, OperationRVersion: 7})
	require.NoError(t, err)
	assert.Equal(t, first.Token, second.Token)
	rowBeforeStop := loadServiceCodespace(t, codespaceID)
	require.NoError(t, cleanupCredentialsForStatus(t.Context(), rowBeforeStop, codespace_model.StatusStopped))
	_, err = db.GetEngine(t.Context()).ID(rowBeforeStop.ID).Cols("status", "operation_r_version").Update(&codespace_model.Codespace{Status: codespace_model.StatusStopped, OperationRVersion: 8})
	require.NoError(t, err)
	resumed, err := requestRuntimeCredentials(t.Context(), manager, requestRuntimeCredentialsOptions{CodespaceID: codespaceID, OperationRVersion: 8})
	require.NoError(t, err)
	assert.NotEqual(t, first.Token, resumed.Token)
	assert.Equal(t, first.Secrets, resumed.Secrets)
}

func TestRuntimeCredentialsRepairsDamagedRow(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1141)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 8,
	})
	badPlaintext := codespaceTokenPrefix + strings.Repeat("0", 64)
	encrypted, err := secret_module.EncryptSecret(setting.SecretKey, badPlaintext)
	require.NoError(t, err)
	require.NoError(t, db.Insert(t.Context(), &codespace_model.GiteaToken{
		CodespaceID:    loadServiceCodespace(t, codespaceID).ID,
		TokenHash:      "wrong-hash",
		TokenSalt:      "salt",
		TokenLastEight: badPlaintext[len(badPlaintext)-8:],
		TokenEncrypted: encrypted,
	}))

	result, err := requestRuntimeCredentials(t.Context(), manager, requestRuntimeCredentialsOptions{CodespaceID: codespaceID, OperationRVersion: 8})
	require.NoError(t, err)
	assert.NotEqual(t, badPlaintext, result.Token)
	row := loadServiceGiteaToken(t, codespaceID)
	assert.True(t, verifyCodespaceGiteaToken(row, result.Token))

	count, err := db.GetEngine(t.Context()).Where("codespace_id = ?", codespaceID).Count(new(codespace_model.GiteaToken))
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

func TestRuntimeCredentialsRejectsUnavailableState(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1014)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     9,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})

	_, err := requestRuntimeCredentials(t.Context(), manager, requestRuntimeCredentialsOptions{CodespaceID: codespaceID, OperationRVersion: 9})
	require.ErrorIs(t, err, ErrRequestRuntimeAccessStateUnavailable)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_id = ?", codespaceID)
}

func TestRuntimeCredentialsRejectsDisabledCodespace(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.Codespace.Enabled, false))

	manager := insertServiceManager(t)
	codespaceID := int64(1025)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 10,
	})

	_, err := requestRuntimeCredentials(t.Context(), manager, requestRuntimeCredentialsOptions{CodespaceID: codespaceID, OperationRVersion: 10})
	require.ErrorIs(t, err, ErrRequestRuntimeAccessStateUnavailable)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_id = ?", codespaceID)
}

func TestRuntimeCredentialsUsesCurrentManagerAvailability(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	staleManager := *manager
	codespaceID := int64(1142)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 10,
	})
	_, err := db.GetEngine(t.Context()).
		ID(manager.ID).
		Cols("last_online_unix").
		Update(&codespace_model.Manager{LastOnlineUnix: 1})
	require.NoError(t, err)

	_, err = requestRuntimeCredentials(t.Context(), &staleManager, requestRuntimeCredentialsOptions{CodespaceID: codespaceID, OperationRVersion: 10})
	require.ErrorIs(t, err, ErrRequestRuntimeAccessManagerOffline)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_id = ?", codespaceID)

	_, err = db.GetEngine(t.Context()).
		ID(manager.ID).
		Cols("runtime_state", "last_online_unix").
		Update(&codespace_model.Manager{
			RuntimeState:   codespace_model.ManagerRuntimeStateRecovering,
			LastOnlineUnix: time.Now().Unix(),
		})
	require.NoError(t, err)
	result, err := requestRuntimeCredentials(t.Context(), &staleManager, requestRuntimeCredentialsOptions{CodespaceID: codespaceID, OperationRVersion: 10})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Token)
}

func TestResolveGiteaTokenRequiresTwoFactorWhenEnforced(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.TwoFactorAuthEnforced, true))

	token := createRunningServiceGiteaTokenForUser(t, int64(1026), 1)

	snapshot, err := ResolveGiteaToken(t.Context(), token)
	require.ErrorIs(t, err, ErrResolveGiteaTokenForbidden)
	assert.Nil(t, snapshot)
}

func TestResolveGiteaTokenAcceptsTwoFactorOrWebAuthnWhenEnforced(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.TwoFactorAuthEnforced, true))

	tests := []struct {
		name        string
		codespaceID int64
		userID      int64
	}{
		{
			name:        "totp",
			codespaceID: int64(1032),
			userID:      24,
		},
		{
			name:        "webauthn",
			codespaceID: int64(1143),
			userID:      32,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "webauthn" {
				require.NoError(t, db.Insert(t.Context(), &auth_model.WebAuthnCredential{UserID: tt.userID, Name: "second credential"}))
			}
			token := createRunningServiceGiteaTokenForUser(t, tt.codespaceID, tt.userID)
			candidates, err := findGiteaTokenAuthCandidates(t.Context(), token[len(token)-8:])
			require.NoError(t, err)
			require.Len(t, candidates, 1)

			snapshot, err := ResolveGiteaToken(t.Context(), token)
			require.NoError(t, err)
			require.NotNil(t, snapshot)
			assert.Equal(t, tt.userID, snapshot.User.ID)
			assert.Equal(t, tt.codespaceID, snapshot.CodespaceID)
			assert.EqualValues(t, 2, snapshot.RepoID)
		})
	}
}

func TestLoadTokenRepositoryPermissions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	now := time.Now().Unix()
	authorization := &codespace_model.PermissionAuthorization{
		UserID: 1, SourceRepoID: 2, RequestHash: "token-permissions",
		CreatedUnix: now, UpdatedUnix: now,
	}
	require.NoError(t, db.Insert(t.Context(), authorization))
	require.NoError(t, db.Insert(t.Context(),
		&codespace_model.PermissionRepository{
			AuthorizationID: authorization.ID, TargetRepoID: 3, UnitType: unit.TypeCode,
			RequestedMode: perm.AccessModeWrite, GrantedMode: perm.AccessModeRead,
		},
		&codespace_model.PermissionRepository{
			AuthorizationID: authorization.ID, TargetRepoID: 3, UnitType: unit.TypeIssues,
			RequestedMode: perm.AccessModeWrite, GrantedMode: perm.AccessModeWrite,
		},
	))
	codespace := &codespace_model.Codespace{UserID: 1, RepoID: 2, PermissionAuthorizationID: authorization.ID}

	permissions, err := loadTokenRepositoryPermissions(t.Context(), codespace)
	require.NoError(t, err)
	snapshot := &GiteaTokenAuthSnapshot{RepoID: codespace.RepoID, RepositoryPermissions: permissions}
	assert.True(t, snapshot.CodespaceTokenAllowsRepository(3, unit.TypeCode, perm.AccessModeRead))
	assert.False(t, snapshot.CodespaceTokenAllowsRepository(3, unit.TypeCode, perm.AccessModeWrite))
	assert.True(t, snapshot.CodespaceTokenAllowsRepository(3, unit.TypeIssues, perm.AccessModeWrite))
	assert.False(t, snapshot.CodespaceTokenAllowsRepository(3, unit.TypeWiki, perm.AccessModeRead))

	for _, tt := range []struct {
		name         string
		userID       int64
		sourceRepoID int64
		revokedUnix  int64
	}{
		{name: "revoked", userID: 1, sourceRepoID: 2, revokedUnix: now},
		{name: "different-user", userID: 2, sourceRepoID: 2},
		{name: "different-source", userID: 1, sourceRepoID: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			authorization.RevokedUnix = tt.revokedUnix
			_, err := db.GetEngine(t.Context()).ID(authorization.ID).Cols("revoked_unix").Update(authorization)
			require.NoError(t, err)
			codespace.UserID = tt.userID
			codespace.RepoID = tt.sourceRepoID

			permissions, err := loadTokenRepositoryPermissions(t.Context(), codespace)
			require.NoError(t, err)
			assert.Empty(t, permissions)
		})
	}
}

func createRunningServiceGiteaTokenForUser(t *testing.T, codespaceID, userID int64) string {
	t.Helper()
	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 11,
	})
	_, err := db.GetEngine(t.Context()).
		Where("id = ?", codespaceID).
		Cols("user_id").
		Update(&codespace_model.Codespace{UserID: userID})
	require.NoError(t, err)

	result, err := requestRuntimeCredentials(t.Context(), manager, requestRuntimeCredentialsOptions{CodespaceID: codespaceID, OperationRVersion: 11})
	require.NoError(t, err)
	require.NotNil(t, result)
	return result.Token
}

func loadServiceGiteaToken(t *testing.T, codespaceID int64) *codespace_model.GiteaToken {
	t.Helper()
	row := new(codespace_model.GiteaToken)
	has, err := db.GetEngine(t.Context()).ID(loadServiceCodespace(t, codespaceID).ID).Get(row)
	require.NoError(t, err)
	require.True(t, has)
	return row
}
