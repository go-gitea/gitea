// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"os"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

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

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo10 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})

	t.Run("Success", func(t *testing.T) {
		// a successful fork

		fork, err := ForkRepository(t.Context(), user2, user2, ForkRepoOptions{
			BaseRepo: repo10,
			Name:     "test",
		})
		assert.NoError(t, err)
		assert.NotNil(t, fork)

		exist, err := git.IsRepositoryExist(t.Context(), gitrepo.CodeRepoByName(user2.Name, "test"))
		assert.NoError(t, err)
		assert.True(t, exist)

		err = DeleteRepositoryDirectly(t.Context(), fork.ID)
		assert.NoError(t, err)
	})
	t.Run("Failure", func(t *testing.T) {
		// a failed creating because some mock data
		// create the repository directory so that the creation will fail after database record created.
		testFailureRepoName := "test"
		testFailureRepo := gitrepo.CodeRepoByName(user2.Name, testFailureRepoName)
		testFailurePath := gitrepo.RepoLocalPath(testFailureRepo)
		assert.NoError(t, os.MkdirAll(testFailurePath, os.ModePerm))

		forkFailure, err := ForkRepository(t.Context(), user2, user2, ForkRepoOptions{
			BaseRepo: repo10,
			Name:     testFailureRepoName,
		})
		assert.Nil(t, forkFailure)
		assert.Error(t, err)

		// assert the cleanup is successful
		unittest.AssertNotExistsBean(t, &repo_model.Repository{OwnerName: user2.Name, Name: testFailureRepoName})

		exist, err := git.IsRepositoryExist(t.Context(), testFailureRepo)
		assert.NoError(t, err)
		assert.False(t, exist)
	})
}
