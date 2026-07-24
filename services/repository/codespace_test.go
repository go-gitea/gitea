// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
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

	codespaceUUID := codespace_model.NewUUID()
	require.NoError(t, db.Insert(t.Context(), &codespace_model.Codespace{
		UUID:                   codespaceUUID,
		UserID:                 user.ID,
		RepoID:                 repo.ID,
		RefType:                "branch",
		RefName:                "main",
		RepoTag:                "default",
		GitProtocol:            codespace_model.GitProtocolHTTP,
		CommitSHA:              "0123456789012345678901234567890123456789",
		Status:                 codespace_model.StatusRunning,
		OperationRVersion:      2,
		CreatedUnix:            100,
		UpdatedUnix:            200,
		LastActiveUnix:         150,
		AutoStopMode:           codespace_model.AutoStopModeDefault,
		LogFilename:            "codespace_log/" + codespaceUUID + ".log",
		InteractionGeneration:  3,
		RuntimeGeneration:      4,
		OperationCreatedUnix:   0,
		OperationStartedUnix:   0,
		OperationDeadlineUnix:  0,
		AutoStopTimeoutSeconds: 0,
	}))
	require.NoError(t, db.Insert(t.Context(), &codespace_model.GiteaToken{
		CodespaceUUID:  codespaceUUID,
		TokenHash:      "codespace-token-hash",
		TokenSalt:      "salt",
		TokenLastEight: "87654321",
		TokenEncrypted: "encrypted",
		CreatedUnix:    110,
	}))

	require.NoError(t, DeleteRepositoryDirectly(t.Context(), repo.ID))

	unittest.AssertNotExistsBean(t, &repo_model.Repository{ID: repo.ID})
	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(t.Context()).ID(codespaceUUID).Get(codespace)
	require.NoError(t, err)
	require.True(t, has)
	assert.Zero(t, codespace.RepoID)
	assert.Equal(t, codespace_model.StatusRunning, codespace.Status)
	assert.EqualValues(t, 200, codespace.UpdatedUnix)
	assert.EqualValues(t, 3, codespace.InteractionGeneration)
	assert.EqualValues(t, 4, codespace.RuntimeGeneration)
	token := new(codespace_model.GiteaToken)
	has, err = db.GetEngine(t.Context()).ID(codespaceUUID).Get(token)
	require.NoError(t, err)
	require.True(t, has)
}
