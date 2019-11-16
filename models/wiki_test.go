// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/git"
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

func TestRepository_InitWiki(t *testing.T) {
	PrepareTestEnv(t)
	// repo1 already has a wiki
	repo1 := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	assert.NoError(t, repo1.InitWiki())

	// repo2 does not already have a wiki
	repo2 := AssertExistsAndLoadBean(t, &Repository{ID: 2}).(*Repository)
	assert.NoError(t, repo2.InitWiki())
	assert.True(t, repo2.HasWiki())
}

func TestRepository_AddWikiPage(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	const wikiContent = "This is the wiki content"
	const commitMsg = "Commit message"
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	doer := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	for _, wikiName := range []string{
		"Another page",
		"Here's a <tag> and a/slash",
	} {
		wikiName := wikiName
		t.Run("test wiki exist: "+wikiName, func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, repo.AddWikiPage(doer, wikiName, wikiContent, commitMsg))
			// Now need to show that the page has been added:
			gitRepo, err := git.OpenRepository(repo.WikiPath())
			assert.NoError(t, err)
			defer gitRepo.Close()
			masterTree, err := gitRepo.GetTree("master")
			assert.NoError(t, err)
			wikiPath := WikiNameToFilename(wikiName)
			entry, err := masterTree.GetTreeEntryByPath(wikiPath)
			assert.NoError(t, err)
			assert.Equal(t, wikiPath, entry.Name(), "%s not addded correctly", wikiName)
		})
	}

	t.Run("check wiki already exist", func(t *testing.T) {
		t.Parallel()
		// test for already-existing wiki name
		err := repo.AddWikiPage(doer, "Home", wikiContent, commitMsg)
		assert.Error(t, err)
		assert.True(t, IsErrWikiAlreadyExist(err))
	})

	t.Run("check wiki reserved name", func(t *testing.T) {
		t.Parallel()
		// test for reserved wiki name
		err := repo.AddWikiPage(doer, "_edit", wikiContent, commitMsg)
		assert.Error(t, err)
		assert.True(t, IsErrWikiReservedName(err))
	})
}

func TestRepository_EditWikiPage(t *testing.T) {
	const newWikiContent = "This is the new content"
	const commitMsg = "Commit message"
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	doer := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	for _, newWikiName := range []string{
		"Home", // same name as before
		"New home",
		"New/name/with/slashes",
	} {
		PrepareTestEnv(t)
		assert.NoError(t, repo.EditWikiPage(doer, "Home", newWikiName, newWikiContent, commitMsg))

		// Now need to show that the page has been added:
		gitRepo, err := git.OpenRepository(repo.WikiPath())
		assert.NoError(t, err)
		masterTree, err := gitRepo.GetTree("master")
		assert.NoError(t, err)
		wikiPath := WikiNameToFilename(newWikiName)
		entry, err := masterTree.GetTreeEntryByPath(wikiPath)
		assert.NoError(t, err)
		assert.Equal(t, wikiPath, entry.Name(), "%s not editted correctly", newWikiName)

		if newWikiName != "Home" {
			_, err := masterTree.GetTreeEntryByPath("Home.md")
			assert.Error(t, err)
		}
		gitRepo.Close()
	}
}

func TestRepository_DeleteWikiPage(t *testing.T) {
	PrepareTestEnv(t)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	doer := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	assert.NoError(t, repo.DeleteWikiPage(doer, "Home"))

	// Now need to show that the page has been added:
	gitRepo, err := git.OpenRepository(repo.WikiPath())
	assert.NoError(t, err)
	defer gitRepo.Close()
	masterTree, err := gitRepo.GetTree("master")
	assert.NoError(t, err)
	wikiPath := WikiNameToFilename("Home")
	_, err = masterTree.GetTreeEntryByPath(wikiPath)
	assert.Error(t, err)
}
