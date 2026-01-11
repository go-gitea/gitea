// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"fmt"
	"testing"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestLock(t *testing.T, repo *repo_model.Repository, owner *user_model.User) *LFSLock {
	t.Helper()

	path := fmt.Sprintf("%s-%d-%d", t.Name(), repo.ID, time.Now().UnixNano())
	lock, err := CreateLFSLock(t.Context(), repo, &LFSLock{
		OwnerID: owner.ID,
		Path:    path,
	})
	require.NoError(t, err)
	return lock
}

func TestGetLFSLockByIDAndRepo(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	lockRepo1 := createTestLock(t, repo1, user2)
	lockRepo3 := createTestLock(t, repo3, user4)

	fetched, err := GetLFSLockByIDAndRepo(t.Context(), lockRepo1.ID, repo1.ID)
	require.NoError(t, err)
	assert.Equal(t, lockRepo1.ID, fetched.ID)
	assert.Equal(t, repo1.ID, fetched.RepoID)

	_, err = GetLFSLockByIDAndRepo(t.Context(), lockRepo1.ID, repo3.ID)
	assert.Error(t, err)
	assert.True(t, IsErrLFSLockNotExist(err))

	_, err = GetLFSLockByIDAndRepo(t.Context(), lockRepo3.ID, repo1.ID)
	assert.Error(t, err)
	assert.True(t, IsErrLFSLockNotExist(err))
}

func TestDeleteLFSLockByIDRequiresRepoMatch(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	lockRepo1 := createTestLock(t, repo1, user2)
	lockRepo3 := createTestLock(t, repo3, user4)

	_, err := DeleteLFSLockByID(t.Context(), lockRepo3.ID, repo1, user2, true)
	assert.Error(t, err)
	assert.True(t, IsErrLFSLockNotExist(err))

	existing, err := GetLFSLockByIDAndRepo(t.Context(), lockRepo3.ID, repo3.ID)
	require.NoError(t, err)
	assert.Equal(t, lockRepo3.ID, existing.ID)

	deleted, err := DeleteLFSLockByID(t.Context(), lockRepo3.ID, repo3, user4, true)
	require.NoError(t, err)
	assert.Equal(t, lockRepo3.ID, deleted.ID)

	deleted, err = DeleteLFSLockByID(t.Context(), lockRepo1.ID, repo1, user2, false)
	require.NoError(t, err)
	assert.Equal(t, lockRepo1.ID, deleted.ID)
}
