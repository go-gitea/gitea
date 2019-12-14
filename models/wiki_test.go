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
	for _, badFilename := range []string{
		"nofileextension",
		"wrongfileextension.txt",
	} {
		_, err := WikiFilenameToName(badFilename)
		assert.Error(t, err)
		assert.True(t, IsErrWikiInvalidFileName(err))
	}
	_, err := WikiFilenameToName("badescaping%%.md")
	assert.Error(t, err)
	assert.False(t, IsErrWikiInvalidFileName(err))
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
	PrepareTestEnv(t)
	repo1 := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	assert.True(t, repo1.HasWiki())
	repo2 := AssertExistsAndLoadBean(t, &Repository{ID: 2}).(*Repository)
	assert.False(t, repo2.HasWiki())
}
