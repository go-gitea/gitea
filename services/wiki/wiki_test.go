// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package wiki

import (
	"math/rand"
	"path/filepath"
	"strings"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: filepath.Join("..", ".."),
	})
}

func TestWebPathSegments(t *testing.T) {
	a := WebPathSegments("a%2Fa/b+c/d-e/f-g.-")
	assert.EqualValues(t, []string{"a/a", "b c", "d e", "f-g"}, a)
}

func TestUserTitleToWebPath(t *testing.T) {
	type test struct {
		Expected  string
		UserTitle string
	}
	for _, test := range []test{
		{"unnamed", ""},
		{"unnamed", "."},
		{"unnamed", ".."},
		{"wiki-name", "wiki name"},
		{"title.md.-", "title.md"},
		{"wiki-name.-", "wiki-name"},
		{"the+wiki-name.-", "the wiki-name"},
		{"a%2Fb", "a/b"},
		{"a%25b", "a%b"},
	} {
		assert.EqualValues(t, test.Expected, UserTitleToWebPath("", test.UserTitle))
	}
}

func TestWebPathToDisplayName(t *testing.T) {
	type test struct {
		Expected string
		WebPath  WebPath
	}
	for _, test := range []test{
		{"wiki name", "wiki-name"},
		{"wiki-name", "wiki-name.-"},
		{"name with / slash", "name-with %2F slash"},
		{"name with % percent", "name-with %25 percent"},
		{"2000-01-02 meeting", "2000-01-02+meeting.-.md"},
		{"a b", "a%20b.md"},
	} {
		_, displayName := WebPathToUserTitle(test.WebPath)
		assert.EqualValues(t, test.Expected, displayName)
	}
}

func TestWebPathToGitPath(t *testing.T) {
	type test struct {
		Expected string
		WikiName WebPath
	}
	for _, test := range []test{
		{"wiki-name.md", "wiki%20name"},
		{"wiki-name.md", "wiki+name"},
		{"wiki name.md", "wiki%20name.md"},
		{"wiki%20name.md", "wiki%2520name.md"},
		{"2000-01-02-meeting.md", "2000-01-02+meeting"},
		{"2000-01-02 meeting.-.md", "2000-01-02%20meeting.-"},
	} {
		assert.EqualValues(t, test.Expected, WebPathToGitPath(test.WikiName))
	}
}

func TestGitPathToWebPath(t *testing.T) {
	type test struct {
		Expected string
		Filename string
	}
	for _, test := range []test{
		{"hello-world", "hello-world.md"}, // this shouldn't happen, because it should always have a ".-" suffix
		{"hello-world", "hello world.md"},
		{"hello-world.-", "hello-world.-.md"},
		{"hello+world.-", "hello world.-.md"},
		{"symbols-%2F", "symbols %2F.md"},
	} {
		name, err := GitPathToWebPath(test.Filename)
		assert.NoError(t, err)
		assert.EqualValues(t, test.Expected, name)
	}
	for _, badFilename := range []string{
		"nofileextension",
		"wrongfileextension.txt",
	} {
		_, err := GitPathToWebPath(badFilename)
		assert.Error(t, err)
		assert.True(t, repo_model.IsErrWikiInvalidFileName(err))
	}
	_, err := GitPathToWebPath("badescaping%%.md")
	assert.Error(t, err)
	assert.False(t, repo_model.IsErrWikiInvalidFileName(err))
}

func TestUserWebGitPathConsistency(t *testing.T) {
	maxLen := 20
	b := make([]byte, maxLen)
	for i := 0; i < 1000; i++ {
		l := rand.Intn(maxLen)
		for j := 0; j < l; j++ {
			r := rand.Intn(0x80-0x20) + 0x20
			b[j] = byte(r)
		}

		userTitle := strings.TrimSpace(string(b[:l]))
		if userTitle == "" || userTitle == "." || userTitle == ".." {
			continue
		}
		webPath := UserTitleToWebPath("", userTitle)
		gitPath := WebPathToGitPath(webPath)

		webPath1, _ := GitPathToWebPath(gitPath)
		_, userTitle1 := WebPathToUserTitle(webPath1)
		gitPath1 := WebPathToGitPath(webPath1)

		assert.EqualValues(t, userTitle, userTitle1, "UserTitle for userTitle: %q", userTitle)
		assert.EqualValues(t, webPath, webPath1, "WebPath for userTitle: %q", userTitle)
		assert.EqualValues(t, gitPath, gitPath1, "GitPath for userTitle: %q", userTitle)
	}
}

func TestRepository_InitWiki(t *testing.T) {
	unittest.PrepareTestEnv(t)
	// repo1 already has a wiki
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.NoError(t, InitWiki(git.DefaultContext, repo1))

	// repo2 does not already have a wiki
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	assert.NoError(t, InitWiki(git.DefaultContext, repo2))
	assert.True(t, repo2.HasWiki())
}

func TestRepository_AddWikiPage(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	const wikiContent = "This is the wiki content"
	const commitMsg = "Commit message"
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	for _, userTitle := range []string{
		"Another page",
		"Here's a <tag> and a/slash",
	} {
		t.Run("test wiki exist: "+userTitle, func(t *testing.T) {
			webPath := UserTitleToWebPath("", userTitle)
			assert.NoError(t, AddWikiPage(git.DefaultContext, doer, repo, webPath, wikiContent, commitMsg))
			// Now need to show that the page has been added:
			gitRepo, err := git.OpenRepository(git.DefaultContext, repo.WikiPath())
			assert.NoError(t, err)
			defer gitRepo.Close()
			masterTree, err := gitRepo.GetTree(DefaultBranch)
			assert.NoError(t, err)
			gitPath := WebPathToGitPath(webPath)
			entry, err := masterTree.GetTreeEntryByPath(gitPath)
			assert.NoError(t, err)
			assert.EqualValues(t, gitPath, entry.Name(), "%s not added correctly", userTitle)
		})
	}

	t.Run("check wiki already exist", func(t *testing.T) {
		t.Parallel()
		// test for already-existing wiki name
		err := AddWikiPage(git.DefaultContext, doer, repo, "Home", wikiContent, commitMsg)
		assert.Error(t, err)
		assert.True(t, repo_model.IsErrWikiAlreadyExist(err))
	})

	t.Run("check wiki reserved name", func(t *testing.T) {
		t.Parallel()
		// test for reserved wiki name
		err := AddWikiPage(git.DefaultContext, doer, repo, "_edit", wikiContent, commitMsg)
		assert.Error(t, err)
		assert.True(t, repo_model.IsErrWikiReservedName(err))
	})
}

func TestRepository_EditWikiPage(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const newWikiContent = "This is the new content"
	const commitMsg = "Commit message"
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	for _, newWikiName := range []string{
		"Home", // same name as before
		"New home",
		"New/name/with/slashes",
	} {
		webPath := UserTitleToWebPath("", newWikiName)
		unittest.PrepareTestEnv(t)
		assert.NoError(t, EditWikiPage(git.DefaultContext, doer, repo, "Home", webPath, newWikiContent, commitMsg))

		// Now need to show that the page has been added:
		gitRepo, err := git.OpenRepository(git.DefaultContext, repo.WikiPath())
		assert.NoError(t, err)
		masterTree, err := gitRepo.GetTree(DefaultBranch)
		assert.NoError(t, err)
		gitPath := WebPathToGitPath(webPath)
		entry, err := masterTree.GetTreeEntryByPath(gitPath)
		assert.NoError(t, err)
		assert.EqualValues(t, gitPath, entry.Name(), "%s not edited correctly", newWikiName)

		if newWikiName != "Home" {
			_, err := masterTree.GetTreeEntryByPath("Home.md")
			assert.Error(t, err)
		}
		gitRepo.Close()
	}
}

func TestRepository_DeleteWikiPage(t *testing.T) {
	unittest.PrepareTestEnv(t)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	assert.NoError(t, DeleteWikiPage(git.DefaultContext, doer, repo, "Home"))

	// Now need to show that the page has been added:
	gitRepo, err := git.OpenRepository(git.DefaultContext, repo.WikiPath())
	assert.NoError(t, err)
	defer gitRepo.Close()
	masterTree, err := gitRepo.GetTree(DefaultBranch)
	assert.NoError(t, err)
	gitPath := WebPathToGitPath("Home")
	_, err = masterTree.GetTreeEntryByPath(gitPath)
	assert.Error(t, err)
}

func TestPrepareWikiFileName(t *testing.T) {
	unittest.PrepareTestEnv(t)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	gitRepo, err := git.OpenRepository(git.DefaultContext, repo.WikiPath())
	assert.NoError(t, err)
	defer gitRepo.Close()

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
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webPath := UserTitleToWebPath("", tt.arg)
			existence, newWikiPath, err := prepareGitPath(gitRepo, webPath)
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
			assert.EqualValues(t, tt.wikiPath, newWikiPath)
		})
	}
}

func TestPrepareWikiFileName_FirstPage(t *testing.T) {
	unittest.PrepareTestEnv(t)

	// Now create a temporaryDirectory
	tmpDir := t.TempDir()

	err := git.InitRepository(git.DefaultContext, tmpDir, true)
	assert.NoError(t, err)

	gitRepo, err := git.OpenRepository(git.DefaultContext, tmpDir)
	assert.NoError(t, err)
	defer gitRepo.Close()

	existence, newWikiPath, err := prepareGitPath(gitRepo, "Home")
	assert.False(t, existence)
	assert.NoError(t, err)
	assert.EqualValues(t, "Home.md", newWikiPath)
}
