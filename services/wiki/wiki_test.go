// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package wiki

import (
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: filepath.Join("..", ".."),
	})
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
		assert.Equal(t, test.Expected, NameToSubURL(test.WikiName))
	}
}

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
		assert.Equal(t, test.Expected, NameToFilename(test.WikiName))
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
		name, err := FilenameToName(test.Filename)
		assert.NoError(t, err)
		assert.Equal(t, test.Expected, name)
	}
	for _, badFilename := range []string{
		"nofileextension",
		"wrongfileextension.txt",
	} {
		_, err := FilenameToName(badFilename)
		assert.Error(t, err)
		assert.True(t, models.IsErrWikiInvalidFileName(err))
	}
	_, err := FilenameToName("badescaping%%.md")
	assert.Error(t, err)
	assert.False(t, models.IsErrWikiInvalidFileName(err))
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
		filename := NameToFilename(name)
		resultName, err := FilenameToName(filename)
		assert.NoError(t, err)
		assert.Equal(t, NormalizeWikiName(name), resultName)
	}
}

func TestRepository_InitWiki(t *testing.T) {
	unittest.PrepareTestEnv(t)
	// repo1 already has a wiki
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	assert.NoError(t, InitWiki(git.DefaultContext, repo1))

	// repo2 does not already have a wiki
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2}).(*repo_model.Repository)
	assert.NoError(t, InitWiki(git.DefaultContext, repo2))
	assert.True(t, repo2.HasWiki())
}

func TestRepository_AddWikiPage(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	const wikiContent = "This is the wiki content"
	const commitMsg = "Commit message"
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	for _, wikiName := range []string{
		"Another page",
		"Here's a <tag> and a/slash",
	} {
		wikiName := wikiName
		t.Run("test wiki exist: "+wikiName, func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, AddWikiPage(git.DefaultContext, doer, repo, wikiName, wikiContent, commitMsg))
			// Now need to show that the page has been added:
			gitRepo, err := git.OpenRepository(git.DefaultContext, repo.WikiPath())
			assert.NoError(t, err)
			defer gitRepo.Close()
			masterTree, err := gitRepo.GetTree("master")
			assert.NoError(t, err)
			wikiPath := NameToFilename(wikiName)
			entry, err := masterTree.GetTreeEntryByPath(wikiPath)
			assert.NoError(t, err)
			assert.Equal(t, wikiPath, entry.Name(), "%s not added correctly", wikiName)
		})
	}

	t.Run("check wiki already exist", func(t *testing.T) {
		t.Parallel()
		// test for already-existing wiki name
		err := AddWikiPage(git.DefaultContext, doer, repo, "Home", wikiContent, commitMsg)
		assert.Error(t, err)
		assert.True(t, models.IsErrWikiAlreadyExist(err))
	})

	t.Run("check wiki reserved name", func(t *testing.T) {
		t.Parallel()
		// test for reserved wiki name
		err := AddWikiPage(git.DefaultContext, doer, repo, "_edit", wikiContent, commitMsg)
		assert.Error(t, err)
		assert.True(t, models.IsErrWikiReservedName(err))
	})
}

func TestRepository_EditWikiPage(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const newWikiContent = "This is the new content"
	const commitMsg = "Commit message"
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	for _, newWikiName := range []string{
		"Home", // same name as before
		"New home",
		"New/name/with/slashes",
	} {
		unittest.PrepareTestEnv(t)
		assert.NoError(t, EditWikiPage(git.DefaultContext, doer, repo, "Home", newWikiName, newWikiContent, commitMsg))

		// Now need to show that the page has been added:
		gitRepo, err := git.OpenRepository(git.DefaultContext, repo.WikiPath())
		assert.NoError(t, err)
		masterTree, err := gitRepo.GetTree("master")
		assert.NoError(t, err)
		wikiPath := NameToFilename(newWikiName)
		entry, err := masterTree.GetTreeEntryByPath(wikiPath)
		assert.NoError(t, err)
		assert.Equal(t, wikiPath, entry.Name(), "%s not edited correctly", newWikiName)

		if newWikiName != "Home" {
			_, err := masterTree.GetTreeEntryByPath("Home.md")
			assert.Error(t, err)
		}
		gitRepo.Close()
	}
}

func TestRepository_DeleteWikiPage(t *testing.T) {
	unittest.PrepareTestEnv(t)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	assert.NoError(t, DeleteWikiPage(git.DefaultContext, doer, repo, "Home"))

	// Now need to show that the page has been added:
	gitRepo, err := git.OpenRepository(git.DefaultContext, repo.WikiPath())
	assert.NoError(t, err)
	defer gitRepo.Close()
	masterTree, err := gitRepo.GetTree("master")
	assert.NoError(t, err)
	wikiPath := NameToFilename("Home")
	_, err = masterTree.GetTreeEntryByPath(wikiPath)
	assert.Error(t, err)
}

func TestPrepareWikiFileName(t *testing.T) {
	unittest.PrepareTestEnv(t)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	gitRepo, err := git.OpenRepository(git.DefaultContext, repo.WikiPath())
	defer gitRepo.Close()
	assert.NoError(t, err)

	tests := []struct {
		name      string
		arg       string
		existence bool
		wikiPath  string
		wantErr   bool
	}{{
		name:      "add suffix",
		arg:       "Home",
		existence: true,
		wikiPath:  "Home.md",
		wantErr:   false,
	}, {
		name:      "test special chars",
		arg:       "home of and & or wiki page!",
		existence: false,
		wikiPath:  "home-of-and-%26-or-wiki-page%21.md",
		wantErr:   false,
	}, {
		name:      "found unescaped cases",
		arg:       "Unescaped File",
		existence: true,
		wikiPath:  "Unescaped File.md",
		wantErr:   false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existence, newWikiPath, err := prepareWikiFileName(gitRepo, tt.arg)
			if (err != nil) != tt.wantErr {
				assert.NoError(t, err)
				return
			}
			if existence != tt.existence {
				if existence {
					t.Errorf("expect to find no escaped file but we detect one")
				} else {
					t.Errorf("expect to find an escaped file but we could not detect one")
				}
			}
			assert.Equal(t, tt.wikiPath, newWikiPath)
		})
	}
}

func TestPrepareWikiFileName_FirstPage(t *testing.T) {
	unittest.PrepareTestEnv(t)

	// Now create a temporaryDirectory
	tmpDir, err := os.MkdirTemp("", "empty-wiki")
	assert.NoError(t, err)
	defer func() {
		if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
			_ = util.RemoveAll(tmpDir)
		}
	}()

	err = git.InitRepository(git.DefaultContext, tmpDir, true)
	assert.NoError(t, err)

	gitRepo, err := git.OpenRepository(git.DefaultContext, tmpDir)
	defer gitRepo.Close()
	assert.NoError(t, err)

	existence, newWikiPath, err := prepareWikiFileName(gitRepo, "Home")
	assert.False(t, existence)
	assert.NoError(t, err)
	assert.Equal(t, "Home.md", newWikiPath)
}
