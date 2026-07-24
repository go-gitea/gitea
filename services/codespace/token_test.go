// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"strings"
	"testing"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	secret_module "gitea.dev/modules/secret"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestGiteaTokenCreateReturnsStableToken(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
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

	first, err := RequestGiteaToken(t.Context(), manager, RequestGiteaTokenOptions{CodespaceUUID: codespaceUUID})
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.True(t, strings.HasPrefix(first.Token, codespaceTokenPrefix))
	assert.Len(t, first.Token, len(codespaceTokenPrefix)+64)
	assert.NotEmpty(t, first.ServerURL)
	row := loadServiceGiteaToken(t, codespaceUUID)
	assert.Equal(t, first.Token[len(first.Token)-8:], row.TokenLastEight)
	assert.True(t, verifyCodespaceGiteaToken(row, first.Token))

	second, err := RequestGiteaToken(t.Context(), manager, RequestGiteaTokenOptions{CodespaceUUID: codespaceUUID})
	require.NoError(t, err)
	assert.Equal(t, first.Token, second.Token)
}

func TestRequestGiteaTokenRepairsDamagedRow(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
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
		CodespaceUUID:  codespaceUUID,
		TokenHash:      "wrong-hash",
		TokenSalt:      "salt",
		TokenLastEight: badPlaintext[len(badPlaintext)-8:],
		TokenEncrypted: encrypted,
		CreatedUnix:    1,
	}))

	result, err := RequestGiteaToken(t.Context(), manager, RequestGiteaTokenOptions{CodespaceUUID: codespaceUUID})
	require.NoError(t, err)
	assert.NotEqual(t, badPlaintext, result.Token)
	row := loadServiceGiteaToken(t, codespaceUUID)
	assert.True(t, verifyCodespaceGiteaToken(row, result.Token))

	count, err := db.GetEngine(t.Context()).Where("codespace_uuid = ?", codespaceUUID).Count(new(codespace_model.GiteaToken))
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

func TestRequestGiteaTokenRejectsUnavailableState(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
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

	_, err := RequestGiteaToken(t.Context(), manager, RequestGiteaTokenOptions{CodespaceUUID: codespaceUUID})
	require.ErrorIs(t, err, ErrRequestGiteaTokenStateUnavailable)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)
}

func TestRequestGiteaTokenRejectsDisabledCodespace(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.Codespace.Enabled, false))

	manager := insertServiceManager(t)
	codespaceUUID := "20202020-2020-4020-8020-202020202020"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 10,
	})

	_, err := RequestGiteaToken(t.Context(), manager, RequestGiteaTokenOptions{CodespaceUUID: codespaceUUID})
	require.ErrorIs(t, err, ErrRequestGiteaTokenStateUnavailable)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)
}

func TestRequestGiteaTokenUsesCurrentManagerAvailability(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
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

	_, err = RequestGiteaToken(t.Context(), &staleManager, RequestGiteaTokenOptions{CodespaceUUID: codespaceUUID})
	require.ErrorIs(t, err, ErrRequestGiteaTokenManagerOffline)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)

	_, err = db.GetEngine(t.Context()).
		ID(manager.ID).
		Cols("runtime_state", "last_online_unix").
		Update(&codespace_model.Manager{
			RuntimeState:   codespace_model.ManagerRuntimeStateRecovering,
			LastOnlineUnix: time.Now().Unix(),
		})
	require.NoError(t, err)
	result, err := RequestGiteaToken(t.Context(), &staleManager, RequestGiteaTokenOptions{CodespaceUUID: codespaceUUID})
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

func createRunningServiceGiteaTokenForUser(t *testing.T, codespaceUUID string, userID int64) string {
	t.Helper()
	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 11,
	})
	_, err := db.GetEngine(t.Context()).
		ID(codespaceUUID).
		Cols("user_id").
		Update(&codespace_model.Codespace{UserID: userID})
	require.NoError(t, err)

	result, err := RequestGiteaToken(t.Context(), manager, RequestGiteaTokenOptions{CodespaceUUID: codespaceUUID})
	require.NoError(t, err)
	require.NotNil(t, result)
	return result.Token
}

func loadServiceGiteaToken(t *testing.T, codespaceUUID string) *codespace_model.GiteaToken {
	t.Helper()
	row := new(codespace_model.GiteaToken)
	has, err := db.GetEngine(t.Context()).ID(codespaceUUID).Get(row)
	require.NoError(t, err)
	require.True(t, has)
	return row
}
