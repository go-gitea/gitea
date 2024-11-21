// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

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
	for i := 0; i < total; i++ {
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
	err := checkUnadoptedRepositories(db.DefaultContext, "notauser", []string{"repo"}, unadopted)
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
	err = checkUnadoptedRepositories(db.DefaultContext, userName, []string{repoName, unadoptedRepoName}, unadopted)
	assert.NoError(t, err)
	assert.Equal(t, []string{path.Join(userName, unadoptedRepoName)}, unadopted.repositories)
	//
	// Existing (adopted) repository is not returned
	//
	unadopted = &unadoptedRepositories{start: 0, end: 100}
	err = checkUnadoptedRepositories(db.DefaultContext, userName, []string{repoName}, unadopted)
	assert.NoError(t, err)
	assert.Empty(t, unadopted.repositories)
	assert.Equal(t, 0, unadopted.index)
}

func TestListUnadoptedRepositories_ListOptions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	username := "user2"
	unadoptedList := []string{path.Join(username, "unadopted1"), path.Join(username, "unadopted2")}
	for _, unadopted := range unadoptedList {
		_ = os.Mkdir(path.Join(setting.RepoRootPath, unadopted+".git"), 0o755)
	}

	opts := db.ListOptions{Page: 1, PageSize: 1}
	repoNames, count, err := ListUnadoptedRepositories(db.DefaultContext, "", &opts)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, unadoptedList[0], repoNames[0])

	opts = db.ListOptions{Page: 2, PageSize: 1}
	repoNames, count, err = ListUnadoptedRepositories(db.DefaultContext, "", &opts)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, unadoptedList[1], repoNames[0])
}

func TestAdoptRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	assert.NoError(t, unittest.SyncDirs(filepath.Join(setting.RepoRootPath, "user2", "repo1.git"), filepath.Join(setting.RepoRootPath, "user2", "test-adopt.git")))
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	_, err := AdoptRepository(db.DefaultContext, user2, user2, CreateRepoOptions{Name: "test-adopt"})
	assert.NoError(t, err)
	repoTestAdopt := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: "test-adopt"})
	assert.Equal(t, "sha1", repoTestAdopt.ObjectFormatName)
}
