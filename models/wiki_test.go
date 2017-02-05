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

func TestToWikiPageURL(t *testing.T) {
	assert.Equal(t, "wiki-name", ToWikiPageURL("wiki-name"))
	assert.Equal(t, "wiki-name-with-many-spaces", ToWikiPageURL("wiki name with many spaces"))
}

func TestToWikiPageName(t *testing.T) {
	assert.Equal(t, "wiki name", ToWikiPageName("wiki name"))
	assert.Equal(t, "wiki name", ToWikiPageName("wiki-name"))
	assert.Equal(t, "wiki name", ToWikiPageName("wiki\tname"))
	assert.Equal(t, "wiki name", ToWikiPageName("./.././wiki/name"))
}

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

// TODO TestRepository_HasWiki

// TODO TestRepository_InitWiki

func TestRepository_LocalWikiPath(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	expected := filepath.Join(setting.AppDataPath, "tmp/local-wiki/1")
	assert.Equal(t, expected, repo.LocalWikiPath())
}

// TODO TestRepository_UpdateLocalWiki

// TODO ... (all remaining untested functions)
