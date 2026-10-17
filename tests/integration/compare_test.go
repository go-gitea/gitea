// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"
	"gitea.dev/routers/common"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompareTag(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/compare/v1.1...master")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	selection := htmlDoc.doc.Find(".ui.dropdown.select-branch")
	// A dropdown for both base and head.
	assert.Lenf(t, selection.Nodes, 2, "The template has changed")

	req = NewRequest(t, "GET", "/user2/repo1/compare/v1.1...HEAD")
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.True(t, test.IsNormalPageCompleted(resp.Body.String()))

	req = NewRequest(t, "GET", "/user2/repo1/compare/v1.1...NotExisting").SetHeader("Accept", "text/html")
	resp = session.MakeRequest(t, req, http.StatusNotFound)
	assert.True(t, test.IsNormalPageCompleted(resp.Body.String()))

	req = NewRequest(t, "GET", "/user2/repo1/compare/invalid").SetHeader("Accept", "text/html")
	resp = session.MakeRequest(t, req, http.StatusNotFound)
	assert.True(t, test.IsNormalPageCompleted(resp.Body.String()))
}

// Compare with inferred default branch (master)
func TestCompareDefault(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/compare/v1.1")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	selection := htmlDoc.doc.Find(".ui.dropdown.select-branch")
	assert.Lenf(t, selection.Nodes, 2, "The template has changed")
}

// Ensure the comparison matches what we expect
func inspectCompare(t *testing.T, session *TestSession, htmlDoc *HTMLDoc, diffChanges []string) {
	files, _ := requestDiffStream(t, session, diffViewerConfigOf(t, htmlDoc).StreamURL)
	var names []string
	for _, file := range files {
		names = append(names, file.Name)
	}
	assert.ElementsMatch(t, diffChanges, names)
}

// Git commit graph for repo20
// * 8babce9 (origin/remove-files-b) Add a dummy file
// * b67e43a Delete test.csv and link_hi
// | * cfe3b3c (origin/remove-files-a) Delete test.csv and link_hi
// |/
// * c8e31bc (origin/add-csv) Add test csv file
// * 808038d (HEAD -> master, origin/master, origin/HEAD) Added test links

func TestCompareBranches(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Indirect compare remove-files-b (head) with add-csv (base) branch
	//
	//	'link_hi' and 'test.csv' are deleted, 'test.txt' is added
	req := NewRequest(t, "GET", "/user2/repo20/compare/add-csv...remove-files-b")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	diffChanges := []string{"link_hi", "test.csv", "test.txt"}

	inspectCompare(t, session, htmlDoc, diffChanges)

	// Indirect compare remove-files-b (head) with remove-files-a (base) branch
	//
	//	'link_hi' and 'test.csv' are deleted, 'test.txt' is added

	req = NewRequest(t, "GET", "/user2/repo20/compare/remove-files-a...remove-files-b")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	diffChanges = []string{"link_hi", "test.csv", "test.txt"}

	inspectCompare(t, session, htmlDoc, diffChanges)

	// Indirect compare remove-files-a (head) with remove-files-b (base) branch
	//
	//	'link_hi' and 'test.csv' are deleted

	req = NewRequest(t, "GET", "/user2/repo20/compare/remove-files-b...remove-files-a")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	diffChanges = []string{"link_hi", "test.csv"}

	inspectCompare(t, session, htmlDoc, diffChanges)

	// Direct compare remove-files-b (head) with remove-files-a (base) branch
	//
	//	'test.txt' is deleted

	req = NewRequest(t, "GET", "/user2/repo20/compare/remove-files-b..remove-files-a")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	diffChanges = []string{"test.txt"}

	inspectCompare(t, session, htmlDoc, diffChanges)
}

func TestCompareWithRefSuffix(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	// remove-files-b^ resolves to the tip's parent, so the test.txt added by the tip is excluded
	req := NewRequest(t, "GET", "/user2/repo20/compare/add-csv...remove-files-b^")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	inspectCompare(t, session, htmlDoc, []string{"link_hi", "test.csv"})

	// a suffix resolves to a commit rather than a branch, so the page offers no pull request to create
	assert.Equal(t, 0, htmlDoc.doc.Find(".pullrequest-form").Length())

	// the same suffix on the direct ".." comparison resolves to the same commit
	req = NewRequest(t, "GET", "/user2/repo20/compare/add-csv..remove-files-b^")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	inspectCompare(t, session, htmlDoc, []string{"link_hi", "test.csv"})

	// a ~N suffix on the base side resolves and renders the compare page, but also
	// resolves to a commit rather than a branch, so no pull request form is offered
	req = NewRequest(t, "GET", "/user2/repo20/compare/add-csv~1...remove-files-b")
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.True(t, test.IsNormalPageCompleted(resp.Body.String()))
	htmlDoc = NewHTMLParser(t, resp.Body)
	assert.Equal(t, 0, htmlDoc.doc.Find(".pullrequest-form").Length())

	// the web handler folds an unsupported (^{...}) and an unresolvable (~50) suffix alike into 404
	for _, basehead := range []string{
		"add-csv...remove-files-b~50",
		"add-csv...remove-files-b^{/Add}",
		"add-csv^{/Add}...remove-files-b",
	} {
		req = NewRequest(t, "GET", "/user2/repo20/compare/"+basehead).SetHeader("Accept", "text/html")
		resp = session.MakeRequest(t, req, http.StatusNotFound)
		assert.True(t, test.IsNormalPageCompleted(resp.Body.String()))
	}
}

func TestResolveRefWithSuffixContract(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 31})
	gitRepo, err := git.OpenRepository(t.Context(), repo)
	require.NoError(t, err)
	defer gitRepo.Close()

	// a nil error guarantees a usable RefName
	ref, err := common.ResolveRefWithSuffix(t.Context(), gitRepo, "add-csv", "^")
	require.NoError(t, err)
	assert.NotEmpty(t, ref)
	// a ref resolved with a suffix must be a commit SHA, not a branch ref
	// (branch refs would break "New Pull Request" logic)
	assert.False(t, ref.IsBranch(), "ref with suffix must not resolve to a branch")

	// a missing ref and an unresolvable suffix both report not-found instead of an empty RefName
	for _, tc := range []struct{ oriRef, suffix string }{
		{"does-not-exist", ""},
		{"add-csv", "~50"},
	} {
		ref, err := common.ResolveRefWithSuffix(t.Context(), gitRepo, tc.oriRef, tc.suffix)
		assert.ErrorIs(t, err, util.ErrNotExist, "ref %q suffix %q", tc.oriRef, tc.suffix)
		assert.Empty(t, ref, "ref %q suffix %q", tc.oriRef, tc.suffix)
	}
}

func TestCompareBranchesNoCommonMergeBase(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user2.ID, Name: "repo1"})

	_, _, runErr := gitcmd.NewCommand("fast-import").WithRepo(repo1).WithStdinBytes([]byte(strings.TrimSpace(`
commit refs/heads/unrelated-history
committer User <user@example.com> 1714310400 +0000
data 13
Second commit
M 100644 inline file2.txt
data 12
Hello from 2
`))).RunStdString(t.Context())
	require.NoError(t, runErr)

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/compare/master...unrelated-history")
	resp := session.MakeRequest(t, req, http.StatusOK)
	body := resp.Body.String()
	htmlDoc := NewHTMLParser(t, resp.Body)

	selection := htmlDoc.doc.Find(".ui.dropdown.select-branch")
	assert.Lenf(t, selection.Nodes, 2, "The template has changed")
	assert.Contains(t, body, "These branches do not share a common merge base")
	assert.Equal(t, 1, htmlDoc.doc.Find(`a.item[href="/user2/repo1/compare/master...unrelated-history"]`).Length())
	assert.Equal(t, 1, htmlDoc.doc.Find(`a.item[href="/user2/repo1/compare/master...master"]`).Length())
	assert.Equal(t, 0, htmlDoc.doc.Find(".pullrequest-form").Length())
}

func TestCompareDownloadDiffOrPatch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	t.Run("BranchToBranchDiff", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/user2/repo20/compare/add-csv...remove-files-b.diff")
		resp := session.MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "text/plain; charset=utf-8", resp.Header().Get("Content-Type"))
		assert.Contains(t, resp.Body.String(), "diff --git ")
	})

	t.Run("BranchToBranchPatch", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/user2/repo20/compare/add-csv...remove-files-b.patch")
		resp := session.MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "text/plain; charset=utf-8", resp.Header().Get("Content-Type"))
		assert.True(t, strings.HasPrefix(resp.Body.String(), "From "), "patch output should start with a format-patch header")
	})

	t.Run("SingleRefImplicitBase", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/user2/repo20/compare/add-csv.diff")
		resp := session.MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, resp.Body.String(), "diff --git ")
	})

	t.Run("InvalidBaseRef", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/user2/repo20/compare/does-not-exist...remove-files-b.diff")
		session.MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("PrivateRepoAnonymous", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// repo16 is private; an unauthenticated request must not leak its existence.
		req := NewRequest(t, "GET", "/user2/repo16/compare/master...good-sign.diff")
		MakeRequest(t, req, http.StatusNotFound)
	})
}

func TestCompareCodeExpand(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		repo, err := repo_service.CreateRepositoryDirectly(t.Context(), user1, user1, repo_service.CreateRepoOptions{
			Name:          "test_code_expand",
			Readme:        "Default",
			AutoInit:      true,
			DefaultBranch: "main",
		}, true)
		assert.NoError(t, err)

		session := loginUser(t, user1.Name)
		testEditFile(t, session, user1.Name, repo.Name, "main", "README.md", strings.Repeat("a\n", 30))

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session = loginUser(t, user2.Name)
		testRepoFork(t, session, user1.Name, repo.Name, user2.Name, "test_code_expand-fork", "")
		testCreateBranch(t, session, user2.Name, "test_code_expand-fork", "branch/main", "forked-branch", http.StatusSeeOther)
		testEditFile(t, session, user2.Name, "test_code_expand-fork", "forked-branch", "README.md", strings.Repeat("a\n", 15)+"CHANGED\n"+strings.Repeat("a\n", 15))

		req := NewRequest(t, "GET", "/user1/test_code_expand/compare/main...user2/test_code_expand-fork:forked-branch")
		resp := session.MakeRequest(t, req, http.StatusOK)
		config := diffViewerConfigOf(t, NewHTMLParser(t, resp.Body))

		// the diff and the file contents for expanding are loaded from the forked repo
		assert.True(t, strings.HasPrefix(config.StreamURL, "/user2/test_code_expand-fork/"), config.StreamURL)
		assert.Equal(t, "/user2/test_code_expand-fork", config.RepoLink)
		files, _ := requestDiffStream(t, session, config.StreamURL)
		require.Len(t, files, 1)
		assert.Contains(t, files[0].Hunks, "+CHANGED")
	})
}
