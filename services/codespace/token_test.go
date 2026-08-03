// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"strings"
	"testing"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	secret_module "gitea.dev/modules/secret"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeCredentialsCreateReturnsStableToken(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceUUID := "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     7,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	encryptedSecret, err := secret_module.EncryptSecret(setting.SecretKey, "runtime-value")
	require.NoError(t, err)
	userSecret := &codespace_model.UserSecret{UserID: 1, Name: "RUNTIME_SECRET", DataEncrypted: encryptedSecret, DataSize: 13}
	require.NoError(t, db.Insert(t.Context(), userSecret))
	require.NoError(t, db.Insert(t.Context(), &codespace_model.UserSecretRepository{SecretID: userSecret.ID, RepoID: 2}))

	first, err := requestRuntimeCredentials(t.Context(), manager, requestRuntimeCredentialsOptions{CodespaceUUID: codespaceUUID, OperationRVersion: 7})
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.True(t, strings.HasPrefix(first.Token, codespaceTokenPrefix))
	assert.Len(t, first.Token, len(codespaceTokenPrefix)+64)
	assert.NotEmpty(t, first.ServerURL)
	assert.Equal(t, []RuntimeSecret{{Name: "RUNTIME_SECRET", Value: "runtime-value"}}, first.Secrets)
	row := loadServiceGiteaToken(t, codespaceUUID)
	assert.Equal(t, first.Token[len(first.Token)-8:], row.TokenLastEight)
	assert.True(t, verifyCodespaceGiteaToken(row, first.Token))

	second, err := requestRuntimeCredentials(t.Context(), manager, requestRuntimeCredentialsOptions{CodespaceUUID: codespaceUUID, OperationRVersion: 7})
	require.NoError(t, err)
	assert.Equal(t, first.Token, second.Token)
}

func TestRuntimeCredentialsOmitsUnauthorizedSecrets(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	tests := []struct {
		name            string
		codespaceUUID   string
		userID          int64
		repoID          int64
		refType         string
		refName         string
		allRepositories bool
	}{
		{
			name:          "code write access revoked",
			codespaceUUID: "0a0a0a0a-0a0a-4a0a-8a0a-0a0a0a0a0a0a",
			userID:        5,
			repoID:        2,
			refType:       "branch",
			refName:       "master",
		},
		{
			name:            "external pull request",
			codespaceUUID:   "0b0b0b0b-0b0b-4b0b-8b0b-0b0b0b0b0b0b",
			userID:          12,
			repoID:          10,
			refType:         "pull",
			refName:         "refs/pull/1/head",
			allRepositories: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := insertServiceManager(t)
			markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
			insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
				UUID:              tt.codespaceUUID,
				Status:            codespace_model.StatusRunning,
				OperationRVersion: 1,
			})
			_, err := db.GetEngine(t.Context()).Where("uuid = ?", tt.codespaceUUID).Cols("user_id", "repo_id", "ref_type", "ref_name").Update(&codespace_model.Codespace{
				UserID: tt.userID, RepoID: tt.repoID, RefType: tt.refType, RefName: tt.refName,
			})
			require.NoError(t, err)

			encrypted, err := secret_module.EncryptSecret(setting.SecretKey, "must-not-be-returned")
			require.NoError(t, err)
			secret := &codespace_model.UserSecret{
				UserID: tt.userID, Name: "PRIVATE_VALUE", DataEncrypted: encrypted, DataSize: 20, AllRepositories: tt.allRepositories,
			}
			require.NoError(t, db.Insert(t.Context(), secret))
			if !tt.allRepositories {
				require.NoError(t, db.Insert(t.Context(), &codespace_model.UserSecretRepository{SecretID: secret.ID, RepoID: tt.repoID}))
			}

			result, err := requestRuntimeCredentials(t.Context(), manager, requestRuntimeCredentialsOptions{CodespaceUUID: tt.codespaceUUID, OperationRVersion: 1})
			require.NoError(t, err)
			assert.NotEmpty(t, result.Token)
			assert.Empty(t, result.Secrets)
		})
	}
}

func TestRuntimeCredentialsRepairsDamagedRow(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceUUID := "ffffffff-ffff-4fff-8fff-ffffffffffff"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 8,
	})
	badPlaintext := codespaceTokenPrefix + strings.Repeat("0", 64)
	encrypted, err := secret_module.EncryptSecret(setting.SecretKey, badPlaintext)
	require.NoError(t, err)
	require.NoError(t, db.Insert(t.Context(), &codespace_model.GiteaToken{
		CodespaceID:    loadServiceCodespace(t, codespaceUUID).ID,
		TokenHash:      "wrong-hash",
		TokenSalt:      "salt",
		TokenLastEight: badPlaintext[len(badPlaintext)-8:],
		TokenEncrypted: encrypted,
	}))

	result, err := requestRuntimeCredentials(t.Context(), manager, requestRuntimeCredentialsOptions{CodespaceUUID: codespaceUUID, OperationRVersion: 8})
	require.NoError(t, err)
	assert.NotEqual(t, badPlaintext, result.Token)
	row := loadServiceGiteaToken(t, codespaceUUID)
	assert.True(t, verifyCodespaceGiteaToken(row, result.Token))

	count, err := db.GetEngine(t.Context()).Where("codespace_id = (SELECT id FROM codespace WHERE uuid = ?)", codespaceUUID).Count(new(codespace_model.GiteaToken))
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

func TestRuntimeCredentialsRejectsUnavailableState(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceUUID := "10101010-1010-4010-8010-101010101010"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     9,
		OperationType:         codespace_model.OperationStop,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})

	_, err := requestRuntimeCredentials(t.Context(), manager, requestRuntimeCredentialsOptions{CodespaceUUID: codespaceUUID, OperationRVersion: 9})
	require.ErrorIs(t, err, ErrRequestRuntimeAccessStateUnavailable)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_id = (SELECT id FROM codespace WHERE uuid = ?)", codespaceUUID)
}

func TestRuntimeCredentialsRejectsDisabledCodespace(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.Codespace.Enabled, false))

	manager := insertServiceManager(t)
	codespaceUUID := "20202020-2020-4020-8020-202020202020"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 10,
	})

	_, err := requestRuntimeCredentials(t.Context(), manager, requestRuntimeCredentialsOptions{CodespaceUUID: codespaceUUID, OperationRVersion: 10})
	require.ErrorIs(t, err, ErrRequestRuntimeAccessStateUnavailable)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_id = (SELECT id FROM codespace WHERE uuid = ?)", codespaceUUID)
}

func TestRuntimeCredentialsUsesCurrentManagerAvailability(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	staleManager := *manager
	codespaceUUID := "30303030-3030-4030-8030-303030303031"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 10,
	})
	_, err := db.GetEngine(t.Context()).
		ID(manager.ID).
		Cols("last_online_unix").
		Update(&codespace_model.Manager{LastOnlineUnix: 1})
	require.NoError(t, err)

	_, err = requestRuntimeCredentials(t.Context(), &staleManager, requestRuntimeCredentialsOptions{CodespaceUUID: codespaceUUID, OperationRVersion: 10})
	require.ErrorIs(t, err, ErrRequestRuntimeAccessManagerOffline)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_id = (SELECT id FROM codespace WHERE uuid = ?)", codespaceUUID)

	_, err = db.GetEngine(t.Context()).
		ID(manager.ID).
		Cols("runtime_state", "last_online_unix").
		Update(&codespace_model.Manager{
			RuntimeState:   codespace_model.ManagerRuntimeStateRecovering,
			LastOnlineUnix: time.Now().Unix(),
		})
	require.NoError(t, err)
	result, err := requestRuntimeCredentials(t.Context(), &staleManager, requestRuntimeCredentialsOptions{CodespaceUUID: codespaceUUID, OperationRVersion: 10})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Token)
}

func TestResolveGiteaTokenRequiresTwoFactorWhenEnforced(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.TwoFactorAuthEnforced, true))

	token := createRunningServiceGiteaTokenForUser(t, "30303030-3030-4030-8030-303030303030", 1)

	snapshot, err := ResolveGiteaToken(t.Context(), token)
	require.ErrorIs(t, err, ErrResolveGiteaTokenForbidden)
	assert.Nil(t, snapshot)
}

func TestResolveGiteaTokenAcceptsTwoFactorOrWebAuthnWhenEnforced(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.TwoFactorAuthEnforced, true))

	tests := []struct {
		name          string
		codespaceUUID string
		userID        int64
	}{
		{
			name:          "totp",
			codespaceUUID: "40404040-4040-4040-8040-404040404040",
			userID:        24,
		},
		{
			name:          "webauthn",
			codespaceUUID: "50505050-5050-4050-8050-505050505050",
			userID:        32,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := createRunningServiceGiteaTokenForUser(t, tt.codespaceUUID, tt.userID)

			snapshot, err := ResolveGiteaToken(t.Context(), token)
			require.NoError(t, err)
			require.NotNil(t, snapshot)
			assert.Equal(t, tt.userID, snapshot.User.ID)
			assert.Equal(t, tt.codespaceUUID, snapshot.CodespaceUUID)
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

func createRunningServiceGiteaTokenForUser(t *testing.T, codespaceUUID string, userID int64) string {
	t.Helper()
	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 11,
	})
	_, err := db.GetEngine(t.Context()).
		Where("uuid = ?", codespaceUUID).
		Cols("user_id").
		Update(&codespace_model.Codespace{UserID: userID})
	require.NoError(t, err)

	result, err := requestRuntimeCredentials(t.Context(), manager, requestRuntimeCredentialsOptions{CodespaceUUID: codespaceUUID, OperationRVersion: 11})
	require.NoError(t, err)
	require.NotNil(t, result)
	return result.Token
}

func loadServiceGiteaToken(t *testing.T, codespaceUUID string) *codespace_model.GiteaToken {
	t.Helper()
	row := new(codespace_model.GiteaToken)
	has, err := db.GetEngine(t.Context()).ID(loadServiceCodespace(t, codespaceUUID).ID).Get(row)
	require.NoError(t, err)
	require.True(t, has)
	return row
}
