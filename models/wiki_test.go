// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestRepository_WikiCloneLink(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	cloneLink := repo.WikiCloneLink()
	assert.Equal(t, "ssh://runuser@try.gitea.io:3000/user2/repo1.wiki.git", cloneLink.SSH)
	assert.Equal(t, "https://try.gitea.io/user2/repo1.wiki.git", cloneLink.HTTPS)
}

func TestWikiPath(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	expected := filepath.Join(setting.RepoRootPath, "user2/repo1.wiki.git")
	assert.Equal(t, expected, WikiPath("user2", "repo1"))
}

func TestRepository_WikiPath(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	expected := filepath.Join(setting.RepoRootPath, "user2/repo1.wiki.git")
	assert.Equal(t, expected, repo.WikiPath())
}

func TestRepository_HasWiki(t *testing.T) {
	PrepareTestEnv(t)
	repo1 := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	assert.True(t, repo1.HasWiki())
	repo2 := AssertExistsAndLoadBean(t, &Repository{ID: 2}).(*Repository)
	assert.False(t, repo2.HasWiki())
}
