// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"os"
	"testing"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitrepo"

	"github.com/stretchr/testify/assert"
)

func TestCreateRepositoryDirectly(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// a successful creating repository
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	testRepoName := "created-repo"
	t.Run("Success", func(t *testing.T) {
		createdRepo, err := CreateRepositoryDirectly(t.Context(), user2, user2, CreateRepoOptions{
			Name: testRepoName,
		}, true)
		assert.NoError(t, err)
		assert.NotNil(t, createdRepo)

		exist, err := git.IsRepositoryExist(t.Context(), gitrepo.CodeRepoByName(user2.Name, createdRepo.Name))
		assert.NoError(t, err)
		assert.True(t, exist)

		unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: user2.Name, Name: createdRepo.Name})

		err = DeleteRepositoryDirectly(t.Context(), createdRepo.ID)
		assert.NoError(t, err)
	})

	t.Run("Failure", func(t *testing.T) {
		// a failed creating because some mock data
		// create the repository directory so that the creation will fail after database record created.
		testFailureRepoName := testRepoName
		testFailureRepo := gitrepo.CodeRepoByName(user2.Name, testFailureRepoName)
		testFailurePath := gitrepo.RepoLocalPath(testFailureRepo)
		assert.NoError(t, os.MkdirAll(testFailurePath, os.ModePerm))

		createdRepo2, err := CreateRepositoryDirectly(t.Context(), user2, user2, CreateRepoOptions{
			Name: testFailureRepoName,
		}, true)
		assert.Nil(t, createdRepo2)
		assert.Error(t, err)

		// assert the cleanup is successful
		unittest.AssertNotExistsBean(t, &repo_model.Repository{OwnerName: user2.Name, Name: testFailureRepoName})

		exist, err := git.IsRepositoryExist(t.Context(), testFailureRepo)
		assert.NoError(t, err)
		assert.False(t, exist)
	})
}

func TestCreateRepositoryDirectlyRespectsCurrentRepositoryLimit(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	limit := user.NumRepos + 1
	user.MaxRepoCreation = limit

	_, err := db.GetEngine(t.Context()).Incr("num_repos").ID(user.ID).Update(new(user_model.User))
	assert.NoError(t, err)

	repo, err := CreateRepositoryDirectly(t.Context(), user, user, CreateRepoOptions{Name: "limit-race"}, true)
	assert.Nil(t, repo)
	assert.Error(t, err)
	assert.True(t, repo_model.IsErrReachLimitOfRepo(err))
	unittest.AssertNotExistsBean(t, &repo_model.Repository{OwnerID: user.ID, Name: "limit-race"})
}
