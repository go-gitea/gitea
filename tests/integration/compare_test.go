// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	git_module "code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/test"
	repo_service "code.gitea.io/gitea/services/repository"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
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

	req = NewRequest(t, "GET", "/user2/repo1/compare/invalid").SetHeader("Accept", "text/html")
	resp = session.MakeRequest(t, req, http.StatusNotFound)
	assert.True(t, test.IsNormalPageCompleted(resp.Body.String()), "expect 404 page not 500")
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
func inspectCompare(t *testing.T, htmlDoc *HTMLDoc, diffCount int, diffChanges []string) {
	selection := htmlDoc.doc.Find("#diff-file-boxes").Children()

	assert.Lenf(t, selection.Nodes, diffCount, "Expected %v diffed files, found: %v", diffCount, len(selection.Nodes))

	for _, diffChange := range diffChanges {
		selection = htmlDoc.doc.Find(fmt.Sprintf("[data-new-filename=\"%s\"]", diffChange))
		assert.Lenf(t, selection.Nodes, 1, "Expected 1 match for [data-new-filename=\"%s\"], found: %v", diffChange, len(selection.Nodes))
	}
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

	diffCount := 3
	diffChanges := []string{"link_hi", "test.csv", "test.txt"}

	inspectCompare(t, htmlDoc, diffCount, diffChanges)

	// Indirect compare remove-files-b (head) with remove-files-a (base) branch
	//
	//	'link_hi' and 'test.csv' are deleted, 'test.txt' is added

	req = NewRequest(t, "GET", "/user2/repo20/compare/remove-files-a...remove-files-b")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	diffCount = 3
	diffChanges = []string{"link_hi", "test.csv", "test.txt"}

	inspectCompare(t, htmlDoc, diffCount, diffChanges)

	// Indirect compare remove-files-a (head) with remove-files-b (base) branch
	//
	//	'link_hi' and 'test.csv' are deleted

	req = NewRequest(t, "GET", "/user2/repo20/compare/remove-files-b...remove-files-a")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	diffCount = 2
	diffChanges = []string{"link_hi", "test.csv"}

	inspectCompare(t, htmlDoc, diffCount, diffChanges)

	// Direct compare remove-files-b (head) with remove-files-a (base) branch
	//
	//	'test.txt' is deleted

	req = NewRequest(t, "GET", "/user2/repo20/compare/remove-files-b..remove-files-a")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	diffCount = 1
	diffChanges = []string{"test.txt"}

	inspectCompare(t, htmlDoc, diffCount, diffChanges)
}

func TestCompareCodeExpand(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		repo, err := repo_service.CreateRepositoryDirectly(t.Context(), user1, user1, repo_service.CreateRepoOptions{
			Name:          "test_blob_excerpt",
			Readme:        "Default",
			AutoInit:      true,
			DefaultBranch: "main",
		}, true)
		assert.NoError(t, err)

		session := loginUser(t, user1.Name)
		testEditFile(t, session, user1.Name, repo.Name, "main", "README.md", strings.Repeat("a\n", 30))

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session = loginUser(t, user2.Name)
		testRepoFork(t, session, user1.Name, repo.Name, user2.Name, "test_blob_excerpt-fork", "")
		testCreateBranch(t, session, user2.Name, "test_blob_excerpt-fork", "branch/main", "forked-branch", http.StatusSeeOther)
		testEditFile(t, session, user2.Name, "test_blob_excerpt-fork", "forked-branch", "README.md", strings.Repeat("a\n", 15)+"CHANGED\n"+strings.Repeat("a\n", 15))

		req := NewRequest(t, "GET", "/user1/test_blob_excerpt/compare/main...user2/test_blob_excerpt-fork:forked-branch")
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		els := htmlDoc.Find(`button.code-expander-button[hx-get]`)

		// all the links in the comparison should be to the forked repo&branch
		assert.NotZero(t, els.Length())
		for i := 0; i < els.Length(); i++ {
			link := els.Eq(i).AttrOr("hx-get", "")
			assert.True(t, strings.HasPrefix(link, "/user2/test_blob_excerpt-fork/blob_excerpt/"))
		}
	})
}

func TestCompareRawDiffNormal(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		repo, err := repo_service.CreateRepositoryDirectly(t.Context(), user1, user1, repo_service.CreateRepoOptions{
			Name:          "test_raw_diff",
			Readme:        "Default",
			AutoInit:      true,
			DefaultBranch: "main",
		}, true)
		assert.NoError(t, err)
		session := loginUser(t, user1.Name)

		r, _ := gitrepo.OpenRepository(t.Context(), repo)
		defer r.Close()
		oldRef, _ := r.GetBranchCommit(repo.DefaultBranch)
		oldBlobRef, _ := revParse(r, oldRef.ID.String(), "README.md")

		testEditFile(t, session, user1.Name, repo.Name, "main", "README.md", strings.Repeat("a\n", 2))

		newRef, _ := r.GetBranchCommit(repo.DefaultBranch)
		newBlobRef, _ := revParse(r, newRef.ID.String(), "README.md")

		req := NewRequest(t, "GET", fmt.Sprintf("/user1/test_raw_diff/compare/%s...%s.diff", oldRef.ID.String(), newRef.ID.String()))
		resp := session.MakeRequest(t, req, http.StatusOK)

		expected := fmt.Sprintf(`diff --git a/README.md b/README.md
index %s..%s 100644
--- a/README.md
+++ b/README.md
@@ -1,2 +1,2 @@
-# test_raw_diff
-
+a
+a
`, oldBlobRef[:7], newBlobRef[:7])
		assert.Equal(t, expected, resp.Body.String())
	})
}

func TestCompareRawDiffPatch(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		repo, err := repo_service.CreateRepositoryDirectly(t.Context(), user1, user1, repo_service.CreateRepoOptions{
			Name:          "test_raw_diff",
			Readme:        "Default",
			AutoInit:      true,
			DefaultBranch: "main",
		}, true)
		assert.NoError(t, err)
		session := loginUser(t, user1.Name)

		r, _ := gitrepo.OpenRepository(t.Context(), repo)

		// Get the old commit and blob reference
		oldRef, _ := r.GetBranchCommit(repo.DefaultBranch)
		oldBlobRef, _ := revParse(r, oldRef.ID.String(), "README.md")

		resp := testEditorActionEdit(t, session, user1.Name, repo.Name, "_edit", "main", "README.md", map[string]string{
			"content":       strings.Repeat("a\n", 2),
			"commit_choice": "direct",
		})

		newRef, _ := r.GetBranchCommit(repo.DefaultBranch)
		newBlobRef, _ := revParse(r, newRef.ID.String(), "README.md")

		// Get the last modified time from the response header
		respTs, _ := time.Parse(time.RFC1123, resp.Result().Header.Get("Last-Modified"))
		respTs = respTs.In(time.Local)

		// Format the timestamp to match the expected format in the patch
		customFormat := "Mon, 2 Jan 2006 15:04:05 -0700"
		respTsStr := respTs.Format(customFormat)

		req := NewRequest(t, "GET", fmt.Sprintf("/user1/test_raw_diff/compare/%s...%s.patch", oldRef.ID.String(), newRef.ID.String()))
		resp = session.MakeRequest(t, req, http.StatusOK)

		expected := fmt.Sprintf(`From %s Mon Sep 17 00:00:00 2001
From: User One <user1@example.com>
Date: %s
Subject: [PATCH] Update README.md

---
 README.md | 4 ++--
 1 file changed, 2 insertions(+), 2 deletions(-)

diff --git a/README.md b/README.md
index %s..%s 100644
--- a/README.md
+++ b/README.md
@@ -1,2 +1,2 @@
-# test_raw_diff
-
+a
+a
`, newRef.ID.String(), respTsStr, oldBlobRef[:7], newBlobRef[:7])
		assert.Equal(t, expected, resp.Body.String())
	})
}

func TestCompareRawDiffNormalSameOwnerDifferentRepo(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		repo, err := repo_service.CreateRepositoryDirectly(t.Context(), user1, user1, repo_service.CreateRepoOptions{
			Name:          "test_raw_diff",
			Readme:        "Default",
			AutoInit:      true,
			DefaultBranch: "main",
		}, true)
		assert.NoError(t, err)
		session := loginUser(t, user1.Name)

		headRepo, err := repo_service.CreateRepositoryDirectly(t.Context(), user1, user1, repo_service.CreateRepoOptions{
			Name:          "test_raw_diff_head",
			Readme:        "Default",
			AutoInit:      true,
			DefaultBranch: "main",
		}, true)
		assert.NoError(t, err)

		r, _ := gitrepo.OpenRepository(t.Context(), repo)
		hr, _ := gitrepo.OpenRepository(t.Context(), headRepo)

		oldRef, _ := r.GetBranchCommit(repo.DefaultBranch)
		oldBlobRef, _ := revParse(r, oldRef.ID.String(), "README.md")

		testEditFile(t, session, user1.Name, headRepo.Name, "main", "README.md", strings.Repeat("a\n", 2))

		newRef, _ := hr.GetBranchCommit(headRepo.DefaultBranch)
		newBlobRef, _ := revParse(hr, newRef.ID.String(), "README.md")

		req := NewRequest(t, "GET", fmt.Sprintf("/user1/test_raw_diff/compare/%s...%s/%s:%s.diff", oldRef.ID.String(), user1.LowerName, headRepo.LowerName, newRef.ID.String()))
		resp := session.MakeRequest(t, req, http.StatusOK)

		expected := fmt.Sprintf(`diff --git a/README.md b/README.md
index %s..%s 100644
--- a/README.md
+++ b/README.md
@@ -1,2 +1,2 @@
-# test_raw_diff
-
+a
+a
`, oldBlobRef[:7], newBlobRef[:7])
		assert.Equal(t, expected, resp.Body.String())
	})
}

func TestCompareRawDiffNormalAcrossForks(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		repo, err := repo_service.CreateRepositoryDirectly(t.Context(), user1, user1, repo_service.CreateRepoOptions{
			Name:          "test_raw_diff",
			Readme:        "Default",
			AutoInit:      true,
			DefaultBranch: "main",
		}, true)
		assert.NoError(t, err)

		headRepo, err := repo_service.ForkRepository(t.Context(), user2, user2, repo_service.ForkRepoOptions{
			BaseRepo:     repo,
			Name:         repo.Name,
			Description:  repo.Description,
			SingleBranch: "",
		})
		assert.NoError(t, err)

		session := loginUser(t, user2.Name)

		r, _ := gitrepo.OpenRepository(t.Context(), repo)
		hr, _ := gitrepo.OpenRepository(t.Context(), headRepo)

		oldRef, _ := r.GetBranchCommit(repo.DefaultBranch)
		oldBlobRef, _ := revParse(r, oldRef.ID.String(), "README.md")

		testEditFile(t, session, user2.Name, headRepo.Name, "main", "README.md", strings.Repeat("a\n", 2))

		newRef, _ := hr.GetBranchCommit(headRepo.DefaultBranch)
		newBlobRef, _ := revParse(hr, newRef.ID.String(), "README.md")

		session = loginUser(t, user1.Name)

		req := NewRequest(t, "GET", fmt.Sprintf("/user1/test_raw_diff/compare/%s...%s:%s.diff", oldRef.ID.String(), user2.LowerName, newRef.ID.String()))
		resp := session.MakeRequest(t, req, http.StatusOK)

		expected := fmt.Sprintf(`diff --git a/README.md b/README.md
index %s..%s 100644
--- a/README.md
+++ b/README.md
@@ -1,2 +1,2 @@
-# test_raw_diff
-
+a
+a
`, oldBlobRef[:7], newBlobRef[:7])
		assert.Equal(t, expected, resp.Body.String())
	})
}

// helper function to use rev-parse
// revParse resolves a revision reference to other git-related objects
func revParse(repo *git_module.Repository, ref, file string) (string, error) {
	stdout, _, err := gitcmd.NewCommand("rev-parse").
		AddDynamicArguments(ref + ":" + file).WithDir(repo.Path).
		RunStdString(repo.Ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}
