// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"os"
	"path"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
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
	assert.Equal(t, end-start, len(unadopted.repositories))
}

func TestCheckUnadoptedRepositories(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	//
	// Non existent user
	//
	unadopted := &unadoptedRepositories{start: 0, end: 100}
	err := checkUnadoptedRepositories("notauser", []string{"repo"}, unadopted)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(unadopted.repositories))
	//
	// Unadopted repository is returned
	// Existing (adopted) repository is not returned
	//
	userName := "user2"
	repoName := "repo2"
	unadoptedRepoName := "unadopted"
	unadopted = &unadoptedRepositories{start: 0, end: 100}
	err = checkUnadoptedRepositories(userName, []string{repoName, unadoptedRepoName}, unadopted)
	assert.NoError(t, err)
	assert.Equal(t, []string{path.Join(userName, unadoptedRepoName)}, unadopted.repositories)
	//
	// Existing (adopted) repository is not returned
	//
	unadopted = &unadoptedRepositories{start: 0, end: 100}
	err = checkUnadoptedRepositories(userName, []string{repoName}, unadopted)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(unadopted.repositories))
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
	repoNames, count, err := ListUnadoptedRepositories("", &opts)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, unadoptedList[0], repoNames[0])

	opts = db.ListOptions{Page: 2, PageSize: 1}
	repoNames, count, err = ListUnadoptedRepositories("", &opts)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, unadoptedList[1], repoNames[0])
}
