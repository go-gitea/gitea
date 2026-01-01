// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"path"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"

	"github.com/stretchr/testify/assert"
)

func TestCheckUnadoptedRepositories_Add(t *testing.T) {
	start := 10
	end := 20
	unadopted := &unadoptedRepositories{
		start: start,
		end:   end,
		index: 0,
	}

	total := 30
	for range total {
		unadopted.add("something")
	}

	assert.Equal(t, total, unadopted.index)
	assert.Len(t, unadopted.repositories, end-start)
}

func TestCheckUnadoptedRepositories(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	//
	// Non existent user
	//
	unadopted := &unadoptedRepositories{start: 0, end: 100}
	err := checkUnadoptedRepositories(t.Context(), "notauser", []string{"repo"}, unadopted)
	assert.NoError(t, err)
	assert.Empty(t, unadopted.repositories)
	//
	// Unadopted repository is returned
	// Existing (adopted) repository is not returned
	//
	userName := "user2"
	repoName := "repo2"
	unadoptedRepoName := "unadopted"
	unadopted = &unadoptedRepositories{start: 0, end: 100}
	err = checkUnadoptedRepositories(t.Context(), userName, []string{repoName, unadoptedRepoName}, unadopted)
	assert.NoError(t, err)
	assert.Equal(t, []string{path.Join(userName, unadoptedRepoName)}, unadopted.repositories)
	//
	// Existing (adopted) repository is not returned
	//
	unadopted = &unadoptedRepositories{start: 0, end: 100}
	err = checkUnadoptedRepositories(t.Context(), userName, []string{repoName}, unadopted)
	assert.NoError(t, err)
	assert.Empty(t, unadopted.repositories)
	assert.Equal(t, 0, unadopted.index)
}

func TestListUnadoptedRepositories_ListOptions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	username := "user2"
	unadoptedList := []gitrepo.Repository{
		repo_model.StorageRepo(repo_model.RelativePath(username, "unadopted1")),
		repo_model.StorageRepo(repo_model.RelativePath(username, "unadopted2")),
	}
	for _, unadopted := range unadoptedList {
		_ = gitrepo.CreateRepositoryDir(unadopted)
	}

	opts := db.ListOptions{Page: 1, PageSize: 1}
	repoNames, count, err := ListUnadoptedRepositories(t.Context(), "", &opts)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, unadoptedList[0].RelativePath(), repoNames[0]+".git")

	opts = db.ListOptions{Page: 2, PageSize: 1}
	repoNames, count, err = ListUnadoptedRepositories(t.Context(), "", &opts)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, unadoptedList[1].RelativePath(), repoNames[0]+".git")
}

func TestAdoptRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// a successful adopt
	destRepo := repo_model.StorageRepo(repo_model.RelativePath(user2.Name, "test-adopt"))
	assert.NoError(t, gitrepo.CopyRepository(repo_model.StorageRepo(repo_model.RelativePath(user2.Name, "repo1")), destRepo))

	adoptedRepo, err := AdoptRepository(t.Context(), user2, user2, CreateRepoOptions{Name: "test-adopt"})
	assert.NoError(t, err)
	repoTestAdopt := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: "test-adopt"})
	assert.Equal(t, "sha1", repoTestAdopt.ObjectFormatName)

	// just delete the adopted repo's db records
	err = deleteFailedAdoptRepository(adoptedRepo.ID)
	assert.NoError(t, err)

	unittest.AssertNotExistsBean(t, &repo_model.Repository{OwnerName: user2.Name, Name: "test-adopt"})

	// a failed adopt because some mock data
	// remove the hooks directory and create a file so that we cannot create the hooks successfully
	_ = gitrepo.RemoveRepoFileOrDir(destRepo, "hooks/update.d")
	f, err := gitrepo.CreateRepoFile(destRepo, "hooks/update.d")
	assert.NoError(t, err)
	_, err = f.Write([]byte("tests"))
	assert.NoError(t, err)
	assert.NoError(t, f.Close())

	adoptedRepo, err = AdoptRepository(t.Context(), user2, user2, CreateRepoOptions{Name: "test-adopt"})
	assert.Error(t, err)
	assert.Nil(t, adoptedRepo)

	unittest.AssertNotExistsBean(t, &repo_model.Repository{OwnerName: user2.Name, Name: "test-adopt"})

	exist, err := gitrepo.IsRepositoryExist(repo_model.StorageRepo(repo_model.RelativePath(user2.Name, "test-adopt")))
	assert.NoError(t, err)
	assert.True(t, exist) // the repository should be still in the disk
}
