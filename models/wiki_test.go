// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"path"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/com"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeWikiName(t *testing.T) {
	type test struct {
		Expected string
		WikiName string
	}
	for _, test := range []test{
		{"wiki name", "wiki name"},
		{"wiki name", "wiki-name"},
		{"name with/slash", "name with/slash"},
		{"name with%percent", "name-with%percent"},
		{"%2F", "%2F"},
	} {
		assert.Equal(t, test.Expected, NormalizeWikiName(test.WikiName))
	}
}

func TestWikiNameToFilename(t *testing.T) {
	type test struct {
		Expected string
		WikiName string
	}
	for _, test := range []test{
		{"wiki-name.md", "wiki name"},
		{"wiki-name.md", "wiki-name"},
		{"name-with%2Fslash.md", "name with/slash"},
		{"name-with%25percent.md", "name with%percent"},
	} {
		assert.Equal(t, test.Expected, WikiNameToFilename(test.WikiName))
	}
}

func TestWikiNameToSubURL(t *testing.T) {
	type test struct {
		Expected string
		WikiName string
	}
	for _, test := range []test{
		{"wiki-name", "wiki name"},
		{"wiki-name", "wiki-name"},
		{"name-with%2Fslash", "name with/slash"},
		{"name-with%25percent", "name with%percent"},
	} {
		assert.Equal(t, test.Expected, WikiNameToSubURL(test.WikiName))
	}
}

func TestWikiFilenameToName(t *testing.T) {
	type test struct {
		Expected string
		Filename string
	}
	for _, test := range []test{
		{"hello world", "hello-world.md"},
		{"symbols/?*", "symbols%2F%3F%2A.md"},
	} {
		name, err := WikiFilenameToName(test.Filename)
		assert.NoError(t, err)
		assert.Equal(t, test.Expected, name)
	}
}

func TestWikiNameToFilenameToName(t *testing.T) {
	// converting from wiki name to filename, then back to wiki name should
	// return the original (normalized) name
	for _, name := range []string{
		"wiki-name",
		"wiki name",
		"wiki name with/slash",
		"$$$%%%^^&&!@#$(),.<>",
	} {
		filename := WikiNameToFilename(name)
		resultName, err := WikiFilenameToName(filename)
		assert.NoError(t, err)
		assert.Equal(t, NormalizeWikiName(name), resultName)
	}
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

func TestRepository_HasWiki(t *testing.T) {
	prepareTestEnv(t)
	repo1 := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	assert.True(t, repo1.HasWiki())
	repo2 := AssertExistsAndLoadBean(t, &Repository{ID: 2}).(*Repository)
	assert.False(t, repo2.HasWiki())
}

func TestRepository_InitWiki(t *testing.T) {
	prepareTestEnv(t)
	// repo1 already has a wiki
	repo1 := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	assert.NoError(t, repo1.InitWiki())

	// repo2 does not already have a wiki
	repo2 := AssertExistsAndLoadBean(t, &Repository{ID: 2}).(*Repository)
	assert.NoError(t, repo2.InitWiki())
	assert.True(t, repo2.HasWiki())
}

func TestRepository_LocalWikiPath(t *testing.T) {
	prepareTestEnv(t)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	expected := filepath.Join(setting.AppDataPath, "tmp/local-wiki/1")
	assert.Equal(t, expected, repo.LocalWikiPath())
}

func TestRepository_AddWikiPage(t *testing.T) {
	const wikiContent = "This is the wiki content"
	const commitMsg = "Commit message"
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	doer := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	for _, wikiName := range []string{
		"Another page",
		"Here's a <tag> and a/slash",
	} {
		prepareTestEnv(t)
		assert.NoError(t, repo.AddWikiPage(doer, wikiName, wikiContent, commitMsg))
		expectedPath := path.Join(repo.LocalWikiPath(), WikiNameToFilename(wikiName))
		assert.True(t, com.IsExist(expectedPath))
	}
}

func TestRepository_EditWikiPage(t *testing.T) {
	const newWikiContent = "This is the new content"
	const commitMsg = "Commit message"
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	doer := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	for _, newWikiName := range []string{
		"New home",
		"New/name/with/slashes",
	} {
		prepareTestEnv(t)
		assert.NoError(t, repo.EditWikiPage(doer, "Home", newWikiName, newWikiContent, commitMsg))
		newPath := path.Join(repo.LocalWikiPath(), WikiNameToFilename(newWikiName))
		assert.True(t, com.IsExist(newPath))
		oldPath := path.Join(repo.LocalWikiPath(), "Home.md")
		assert.False(t, com.IsExist(oldPath))
	}
}

func TestRepository_DeleteWikiPage(t *testing.T) {
	prepareTestEnv(t)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	doer := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	assert.NoError(t, repo.DeleteWikiPage(doer, "Home"))
	wikiPath := path.Join(repo.LocalWikiPath(), "Home.md")
	assert.False(t, com.IsExist(wikiPath))
}
