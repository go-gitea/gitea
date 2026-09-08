// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteRepositoryDirectlyUnbindsCodespaces(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo, err := CreateRepositoryDirectly(t.Context(), user, user, CreateRepoOptions{
		Name: "codespace-source",
	}, true)
	require.NoError(t, err)
	require.NotNil(t, repo)
	targetRepo, err := CreateRepositoryDirectly(t.Context(), user, user, CreateRepoOptions{
		Name: "codespace-target",
	}, true)
	require.NoError(t, err)
	require.NotNil(t, targetRepo)
	authorization := &codespace_model.PermissionAuthorization{
		UserID: user.ID, SourceRepoID: repo.ID, RequestHash: "repository-delete",
		CreatedUnix: 100, UpdatedUnix: 100,
	}
	require.NoError(t, db.Insert(t.Context(), authorization))
	rule := &codespace_model.PermissionRepository{
		AuthorizationID: authorization.ID, TargetRepoID: targetRepo.ID, UnitType: unit.TypeCode,
		RequestedMode: perm.AccessModeRead, GrantedMode: perm.AccessModeRead,
	}
	require.NoError(t, db.Insert(t.Context(), rule))
	secret := &codespace_model.UserSecret{UserID: user.ID, Name: "DATABASE_PASSWORD", DataEncrypted: "encrypted", DataSize: 8}
	require.NoError(t, db.Insert(t.Context(), secret))
	targetSecretRepository := &codespace_model.UserSecretRepository{SecretID: secret.ID, RepoID: targetRepo.ID}
	require.NoError(t, db.Insert(t.Context(), targetSecretRepository))
	sourceSecretRepository := &codespace_model.UserSecretRepository{SecretID: secret.ID, RepoID: repo.ID}
	require.NoError(t, db.Insert(t.Context(), sourceSecretRepository))

	codespaceUUID := codespace_model.NewUUID()
	codespace := &codespace_model.Codespace{
		UUID:                      codespaceUUID,
		UserID:                    user.ID,
		RepoID:                    repo.ID,
		PermissionAuthorizationID: authorization.ID,
		RefType:                   "branch",
		RefName:                   "main",
		EnvironmentTag:            "default",
		CommitSHA:                 "0123456789012345678901234567890123456789",
		Status:                    codespace_model.StatusRunning,
		OperationRVersion:         2,
		CreatedUnix:               100,
		UpdatedUnix:               200,
		LastActiveUnix:            150,
		AutoStopMode:              codespace_model.AutoStopModeDefault,
		InteractionGeneration:     3,
		RuntimeGeneration:         4,
		OperationCreatedUnix:      0,
		OperationStartedUnix:      0,
		OperationDeadlineUnix:     0,
		AutoStopTimeoutSeconds:    0,
	}
	require.NoError(t, db.Insert(t.Context(), codespace))
	require.NoError(t, db.Insert(t.Context(), &codespace_model.GiteaToken{
		CodespaceID:    codespace.ID,
		TokenHash:      "codespace-token-hash",
		TokenSalt:      "salt",
		TokenLastEight: "87654321",
		TokenEncrypted: "encrypted",
	}))

	require.NoError(t, DeleteRepositoryDirectly(t.Context(), targetRepo.ID))
	assertRepositoryServiceNotExists(t, new(codespace_model.PermissionRepository), "authorization_id = ? AND target_repo_id = ? AND unit_type = ?", rule.AuthorizationID, rule.TargetRepoID, rule.UnitType)
	unittest.AssertExistsAndLoadBean(t, &codespace_model.PermissionAuthorization{ID: authorization.ID})
	assertRepositoryServiceNotExists(t, new(codespace_model.UserSecretRepository), "secret_id = ? AND repo_id = ?", targetSecretRepository.SecretID, targetSecretRepository.RepoID)
	has, err := db.GetEngine(t.Context()).Where("secret_id = ? AND repo_id = ?", sourceSecretRepository.SecretID, sourceSecretRepository.RepoID).Exist(new(codespace_model.UserSecretRepository))
	require.NoError(t, err)
	assert.True(t, has)

	require.NoError(t, DeleteRepositoryDirectly(t.Context(), repo.ID))

	unittest.AssertNotExistsBean(t, &repo_model.Repository{ID: repo.ID})
	persistedCodespace := new(codespace_model.Codespace)
	has, err = db.GetEngine(t.Context()).Where("uuid = ?", codespaceUUID).Get(persistedCodespace)
	require.NoError(t, err)
	require.True(t, has)
	assert.Zero(t, persistedCodespace.RepoID)
	assert.Zero(t, persistedCodespace.PermissionAuthorizationID)
	assert.Equal(t, codespace_model.StatusRunning, persistedCodespace.Status)
	assert.EqualValues(t, 200, persistedCodespace.UpdatedUnix)
	assert.EqualValues(t, 3, persistedCodespace.InteractionGeneration)
	assert.EqualValues(t, 4, persistedCodespace.RuntimeGeneration)
	token := new(codespace_model.GiteaToken)
	has, err = db.GetEngine(t.Context()).ID(codespace.ID).Get(token)
	require.NoError(t, err)
	require.True(t, has)
	unittest.AssertNotExistsBean(t, &codespace_model.PermissionAuthorization{ID: authorization.ID})
	assertRepositoryServiceNotExists(t, new(codespace_model.UserSecretRepository), "secret_id = ? AND repo_id = ?", sourceSecretRepository.SecretID, sourceSecretRepository.RepoID)
	unittest.AssertExistsAndLoadBean(t, &codespace_model.UserSecret{ID: secret.ID})
}

func assertRepositoryServiceNotExists(t *testing.T, bean any, query string, args ...any) {
	t.Helper()
	has, err := db.GetEngine(t.Context()).Where(query, args...).Exist(bean)
	require.NoError(t, err)
	assert.False(t, has)
}
