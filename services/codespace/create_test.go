// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCodespaceQueuesCreateWhenManagerMatches(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	insertServiceManagerWithTags(t, 0, "default")
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	result, err := CreateCodespace(t.Context(), CreateCodespaceOptions{
		User:    user,
		Repo:    repo,
		RefType: "branch",
		RefName: "master",
	})
	require.NoError(t, err)
	assert.Equal(t, codespace_model.StatusCreating, result.Status)
	assert.Equal(t, "default", result.RepoTag)

	row := loadServiceCodespace(t, result.CodespaceUUID)
	assert.Equal(t, user.ID, row.UserID)
	assert.Equal(t, repo.ID, row.RepoID)
	assert.Equal(t, "branch", row.RefType)
	assert.Equal(t, "master", row.RefName)
	assert.NotEmpty(t, row.CommitSHA)
	assert.Equal(t, codespace_model.GitProtocolHTTP, row.GitProtocol)
	assert.Equal(t, codespace_model.OperationCreate, row.OperationType)
	assert.Equal(t, codespace_model.OperationStatusQueued, row.OperationStatus)
	assert.Equal(t, codespace_model.OperationTriggerUser, row.OperationTrigger)
	assert.EqualValues(t, 1, row.OperationRVersion)
}

func TestCreateCodespaceCreatesFailedWhenNoManagerMatches(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	result, err := CreateCodespace(t.Context(), CreateCodespaceOptions{
		User: user,
		Repo: repo,
	})
	require.NoError(t, err)
	assert.Equal(t, codespace_model.StatusFailed, result.Status)

	row := loadServiceCodespace(t, result.CodespaceUUID)
	assert.Equal(t, codespace_model.StatusFailed, row.Status)
	assert.Zero(t, row.OperationRVersion)
	assert.Empty(t, row.OperationType)
	assert.Empty(t, row.OperationStatus)
	assert.Positive(t, row.LogSize)
}

func TestCreateCodespaceRejectsDisabledCodespace(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.Codespace.Enabled, false))

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	_, err := CreateCodespace(t.Context(), CreateCodespaceOptions{User: user, Repo: repo})
	require.ErrorIs(t, err, ErrCreateStateUnavailable)
}

func TestCreateCodespacePersistsPullRefAndGitProtocol(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureGitTransportTestSettings(t, codespace_model.GitProtocolSSH, false, false, []string{
		"gitea.example.com " + testGitSSHPublicKey,
	})

	insertServiceManagerWithTags(t, 0, "default")
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	result, err := CreateCodespace(t.Context(), CreateCodespaceOptions{
		User:    user,
		Repo:    repo,
		RefType: "pull",
		RefName: "3",
	})
	require.NoError(t, err)

	row := loadServiceCodespace(t, result.CodespaceUUID)
	assert.Equal(t, "pull", row.RefType)
	assert.Equal(t, "refs/pull/3/head", row.RefName)
	assert.Equal(t, codespace_model.GitProtocolSSH, row.GitProtocol)
	assert.NotEmpty(t, row.CommitSHA)
}

func TestCreateCodespaceRejectsInvalidConfig(t *testing.T) {
	for _, content := range [][]byte{
		[]byte("tag: default\ntag: other\n"),
		[]byte("unknown: default\n"),
		[]byte("tag: [default]\n"),
		[]byte("tag: bad tag\n"),
	} {
		_, err := parseCodespaceRepoConfig(content)
		require.Error(t, err)
	}

	tag, err := parseCodespaceRepoConfig([]byte("tag: Custom_Tag\n"))
	require.NoError(t, err)
	assert.Equal(t, "custom_tag", tag)
}

func insertServiceManagerWithTags(t *testing.T, ownerID int64, tags ...string) *codespace_model.Manager {
	t.Helper()
	tagsJSON, err := json.Marshal(tags)
	require.NoError(t, err)
	manager := &codespace_model.Manager{
		Name:           "manager",
		OwnerID:        ownerID,
		RuntimeState:   codespace_model.ManagerRuntimeStateOnline,
		TagsJSON:       string(tagsJSON),
		CreatedUnix:    time.Now().Unix(),
		LastOnlineUnix: time.Now().Unix(),
		MetaJSON:       "{}",
	}
	manager.GenerateManagerSecret()
	require.NoError(t, db.Insert(t.Context(), manager))
	return manager
}
