// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoAssignees(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	users, err := repo_model.GetRepoAssignees(db.DefaultContext, repo2)
	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, int64(2), users[0].ID)

	repo21 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 21})
	users, err = repo_model.GetRepoAssignees(db.DefaultContext, repo21)
	assert.NoError(t, err)
	if assert.Len(t, users, 4) {
		assert.ElementsMatch(t, []int64{10, 15, 16, 18}, []int64{users[0].ID, users[1].ID, users[2].ID, users[3].ID})
	}

	// do not return deactivated users
	assert.NoError(t, user_model.UpdateUserCols(db.DefaultContext, &user_model.User{ID: 15, IsActive: false}, "is_active"))
	users, err = repo_model.GetRepoAssignees(db.DefaultContext, repo21)
	assert.NoError(t, err)
	if assert.Len(t, users, 3) {
		assert.NotContains(t, []int64{users[0].ID, users[1].ID, users[2].ID}, 15)
	}
}

func TestGetIssuePostersWithSearch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})

	users, err := repo_model.GetIssuePostersWithSearch(db.DefaultContext, repo2, false, "USER", false /* full name */)
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, "user2", users[0].Name)

	users, err = repo_model.GetIssuePostersWithSearch(db.DefaultContext, repo2, false, "TW%O", true /* full name */)
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, "user2", users[0].Name)
}
