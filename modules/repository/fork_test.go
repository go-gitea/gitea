// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestCanUserForkBetweenOwners(t *testing.T) {
	defer test.MockVariableValue(&setting.Repository.AllowForkIntoSameOwner)()

	setting.Repository.AllowForkIntoSameOwner = true
	assert.True(t, CanUserForkBetweenOwners(1, 1))
	assert.True(t, CanUserForkBetweenOwners(1, 2))

	setting.Repository.AllowForkIntoSameOwner = false
	assert.False(t, CanUserForkBetweenOwners(1, 1))
	assert.True(t, CanUserForkBetweenOwners(1, 2))
}

func TestCanUserForkRepoIntoOwningOrg(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Repository.AllowForkIntoSameOwner)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	// user2's own fork leaves org3, the owner, as the only target
	assert.NoError(t, db.Insert(t.Context(), &repo_model.Repository{
		OwnerID: 2, Name: "repo3-fork", LowerName: "repo3-fork",
		IsFork: true, ForkID: 3,
	}))

	setting.Repository.AllowForkIntoSameOwner = true
	ok, err := CanUserForkRepo(t.Context(), user2, repo3)
	assert.NoError(t, err)
	assert.True(t, ok)

	setting.Repository.AllowForkIntoSameOwner = false
	ok, err = CanUserForkRepo(t.Context(), user2, repo3)
	assert.NoError(t, err)
	assert.False(t, ok)
}
