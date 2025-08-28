// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"os"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestForkRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// user 13 has already forked repo10
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 13})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})

	fork, err := ForkRepository(t.Context(), user, user, ForkRepoOptions{
		BaseRepo:    repo,
		Name:        "test",
		Description: "test",
	})
	assert.Nil(t, fork)
	assert.Error(t, err)
	assert.True(t, IsErrForkAlreadyExist(err))

	// user not reached maximum limit of repositories
	assert.False(t, repo_model.IsErrReachLimitOfRepo(err))

	// change AllowForkWithoutMaximumLimit to false for the test
	defer test.MockVariableValue(&setting.Repository.AllowForkWithoutMaximumLimit, false)()
	// user has reached maximum limit of repositories
	user.MaxRepoCreation = 0
	fork2, err := ForkRepository(t.Context(), user, user, ForkRepoOptions{
		BaseRepo:    repo,
		Name:        "test",
		Description: "test",
	})
	assert.Nil(t, fork2)
	assert.True(t, repo_model.IsErrReachLimitOfRepo(err))
}

func TestForkRepositoryCleanup(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// a successful fork
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo10 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})

	fork, err := ForkRepository(t.Context(), user2, user2, ForkRepoOptions{
		BaseRepo: repo10,
		Name:     "test",
	})
	assert.NoError(t, err)
	assert.NotNil(t, fork)

	exist, err := util.IsExist(repo_model.RepoPath(user2.Name, "test"))
	assert.NoError(t, err)
	assert.True(t, exist)

	err = DeleteRepositoryDirectly(t.Context(), fork.ID)
	assert.NoError(t, err)

	// a failed creating because some mock data
	// create the repository directory so that the creation will fail after database record created.
	assert.NoError(t, os.MkdirAll(repo_model.RepoPath(user2.Name, "test"), os.ModePerm))

	fork2, err := ForkRepository(t.Context(), user2, user2, ForkRepoOptions{
		BaseRepo: repo10,
		Name:     "test",
	})
	assert.Nil(t, fork2)
	assert.Error(t, err)

	// assert the cleanup is successful
	unittest.AssertNotExistsBean(t, &repo_model.Repository{OwnerName: user2.Name, Name: "test"})

	exist, err = util.IsExist(repo_model.RepoPath(user2.Name, "test"))
	assert.NoError(t, err)
	assert.False(t, exist)
}
