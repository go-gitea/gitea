// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"path/filepath"
	"regexp"
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

func TestWikiNameToSubURL(t *testing.T) {
	type test struct {
		Expected string
		WikiName string
	}
	for _, test := range []test{
		{"wiki%2Fpath", "wiki/../path/../../"},
		{"wiki%2Fpath", " wiki/path ////// "},
		{"wiki-name", "wiki-name"},
		{"name%20with%2Fslash", "name with/slash"},
		{"name%20with%25percent", "name with%percent"},
	} {
		assert.Equal(t, test.Expected, WikiNameToSubURL(test.WikiName))
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
		{"wiki-name-with%2Fslash.md", "wiki name with/slash"},
		{"%24%24%24%25%25%25%5E%5E%26%26%21%40%23%24%28%29%2C.%3C%3E.md", "$$$%%%^^&&!@#$(),.<>"},
	} {
		assert.Equal(t, test.Expected, WikiNameToFilename(test.WikiName))
	}
}

func TestWikiNameToPathFilename(t *testing.T) {
	type test struct {
		Expected string
		WikiName string
	}
	for _, test := range []test{
		{"wiki name.md", "wiki name"},
		{"wiki-name.md", "wiki-name"},
		{"name with/slash.md", "name with/slash"},
		{"name with/slash.md", "name with/../slash"},
		{"name with%percent.md", "name with%percent"},
		{"git/config.md", ".git/config   "},
	} {
		assert.Equal(t, test.Expected, WikiNameToPathFilename(test.WikiName))
	}
}

func TestFilenameToPathFilename(t *testing.T) {
	type test struct {
		Expected string
		Filename string
	}
	for _, test := range []test{
		{"wiki/name.md", "wiki%2Fname.md"},
		{"wiki name path", "wiki%20name+path"},
		{"name with/slash", "name with/slash"},
		{"name with&and", "name with%2526and"},
		{"name with%percent", "name with%percent"},
		{"&&&&", "%26%26%26%26"},
	} {
		assert.Equal(t, test.Expected, FilenameToPathFilename(test.Filename))
	}
}

func TestWikiNameToRawPrefix(t *testing.T) {
	type test struct {
		RepoName string
		WikiPage string
		Expected string
	}
	for _, test := range []test{
		{"/repo1/name", "wiki/path", "/repo1/name/wiki/raw/wiki"},
		{"/repo2/name", "wiki/path/subdir", "/repo2/name/wiki/raw/wiki/path"},
	} {
		assert.Equal(t, test.Expected, WikiNameToRawPrefix(test.RepoName, test.WikiPage))
	}
}

func TestWikiFilenameToName(t *testing.T) {
	type test struct {
		Expected1 string
		Expected2 string
		Filename  string
	}
	for _, test := range []test{
		{"hello world", "hello world", "hello world.md"},
		{"hello-world", "hello-world", "hello-world.md"},
		{"symbols/?*", "symbols%2F%3F%2A", "symbols%2F%3F%2A.md"},
		{"wiki-name-with/slash", "wiki-name-with%2Fslash", "wiki-name-with%2Fslash.md"},
		{"$$$%%%^^&&!@#$(),.<>", "%24%24%24%25%25%25%5E%5E%26%26%21%40%23%24%28%29%2C.%3C%3E", "%24%24%24%25%25%25%5E%5E%26%26%21%40%23%24%28%29%2C.%3C%3E.md"},
	} {
		unescaped, basename, err := WikiFilenameToName(test.Filename)
		assert.NoError(t, err)
		assert.Equal(t, test.Expected1, unescaped)
		assert.Equal(t, test.Expected2, basename)
	}
	for _, badFilename := range []string{
		"nofileextension",
		"wrongfileextension.txt",
	} {
		_, _, err := WikiFilenameToName(badFilename)
		assert.Error(t, err)
		assert.True(t, IsErrWikiInvalidFileName(err))
	}
	_, _, err := WikiFilenameToName("badescaping%%.md")
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
		resultName, _, err := WikiFilenameToName(filename)
		assert.NoError(t, err)
		assert.Equal(t, NormalizeWikiName(name), NormalizeWikiName(resultName))
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
			masterTree, err := gitRepo.GetTree("master")
			assert.NoError(t, err)
			wikiPath := WikiNameToPathFilename(wikiName)
			entry, err := masterTree.GetTreeEntryByPath(wikiPath)
			re := regexp.MustCompile(`(?m)(.*)(\/)([^\/]*)$`)

			assert.NoError(t, err)
			assert.Equal(t, re.ReplaceAllString(wikiPath, "$3"), entry.Name(), "%s not addded correctly", wikiName)
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
		re := regexp.MustCompile(`(?m)(.*)(\/)([^\/]*)$`)

		wikiPath := WikiNameToPathFilename(newWikiName)

		entry, err := masterTree.GetTreeEntryByPath(wikiPath)
		assert.NoError(t, err)
		assert.Equal(t, re.ReplaceAllString(wikiPath, "$3"), entry.Name(), "%s not editted correctly", newWikiName)

		if newWikiName != "Home" {
			_, err := masterTree.GetTreeEntryByPath("Home.md")
			assert.Error(t, err)
		}
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
	masterTree, err := gitRepo.GetTree("master")
	assert.NoError(t, err)
	wikiPath := WikiNameToFilename("Home")
	_, err = masterTree.GetTreeEntryByPath(wikiPath)
	assert.Error(t, err)
}
