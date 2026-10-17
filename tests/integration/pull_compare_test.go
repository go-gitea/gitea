// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/test"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullCompare(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("PullsNewRedirect", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2/repo1/pulls/new/foo")
		resp := MakeRequest(t, req, http.StatusSeeOther)
		redirect := test.RedirectURL(resp)
		assert.Equal(t, "/user2/repo1/compare/master...foo?expand=1", redirect)

		req = NewRequest(t, "GET", "/user13/repo11/pulls/new/foo")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		redirect = test.RedirectURL(resp)
		assert.Equal(t, "/user12/repo10/compare/master...user13:foo?expand=1", redirect)
	})

	t.Run("ButtonsExist", func(t *testing.T) {
		session := loginUser(t, "user2")

		// test the "New PR" button
		req := NewRequest(t, "GET", "/user2/repo1/pulls")
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		link, exists := htmlDoc.doc.Find(".new-pr-button").Attr("href")
		assert.True(t, exists, "The template has changed")
		req = NewRequest(t, "GET", link)
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, http.StatusOK, resp.Code)

		// test the edit button in the PR diff view
		req = NewRequest(t, "GET", "/user2/repo1/pulls/3/files")
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, diffViewerConfigOf(t, NewHTMLParser(t, resp.Body)).EditFileLink, "/_edit/", "Expected the PR diff view to allow editing files")
	})

	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		defer tests.PrepareTestEnv(t)()

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testCreateBranch(t, session, "user1", "repo1", "branch/master", "master1", http.StatusSeeOther)
		testEditFile(t, session, "user1", "repo1", "master1", "README.md", "Hello, World (Edited)\n")
		testPullCreate(t, session, "user1", "repo1", false, "master", "master1", "This is a pull title")

		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		issueIndex := unittest.AssertExistsAndLoadBean(t, &issues_model.IssueIndex{GroupID: repo1.ID}, unittest.OrderBy("group_id ASC"))
		prFilesURL := fmt.Sprintf("/user2/repo1/pulls/%d/files", issueIndex.MaxIndex)
		req := NewRequest(t, "GET", prFilesURL)
		resp := session.MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, diffViewerConfigOf(t, NewHTMLParser(t, resp.Body)).EditFileLink, "/_edit/", "Expected the PR diff view to allow editing files")

		repoForked := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user1", Name: "repo1"})

		// delete the head repository and revisit the PR diff view
		err := repo_service.DeleteRepositoryDirectly(t.Context(), repoForked.ID)
		assert.NoError(t, err)

		req = NewRequest(t, "GET", prFilesURL)
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.Empty(t, diffViewerConfigOf(t, NewHTMLParser(t, resp.Body)).EditFileLink, "Expected the PR diff view not to allow editing files because head repository has been deleted")
	})
}

func TestPullCompare_EnableAllowEditsFromMaintainer(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		// repo3 is private
		repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
		assert.True(t, repo3.IsPrivate)

		// user4 forks repo3
		user4Session := loginUser(t, "user4")
		forkedRepoName := "user4-forked-repo3"
		testRepoFork(t, user4Session, repo3.OwnerName, repo3.Name, "user4", forkedRepoName, "")
		forkedRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user4", Name: forkedRepoName})
		assert.True(t, forkedRepo.IsPrivate)

		// user4 creates a new branch and a PR
		testEditFileToNewBranch(t, user4Session, "user4", forkedRepoName, "master", "user4/update-readme", "README.md", "Hello, World\n(Edited by user4)\n")
		resp := testPullCreateDirectly(t, user4Session, createPullRequestOptions{
			BaseRepoOwner: repo3.OwnerName,
			BaseRepoName:  repo3.Name,
			BaseBranch:    "master",
			HeadRepoOwner: "user4",
			HeadRepoName:  forkedRepoName,
			HeadBranch:    "user4/update-readme",
			Title:         "PR for user4 forked repo3",
		})
		prURL := test.RedirectURL(resp)

		// user2 (admin of repo3) goes to the PR files page, there is no "Edit File" button
		user2Session := loginUser(t, "user2")
		resp = user2Session.MakeRequest(t, NewRequest(t, "GET", prURL+"/files"), http.StatusOK)
		assert.Empty(t, diffViewerConfigOf(t, NewHTMLParser(t, resp.Body)).EditFileLink)

		// user4 goes to the PR page and enable "Allow maintainers to edit"
		resp = user4Session.MakeRequest(t, NewRequest(t, "GET", prURL), http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		dataURL, exists := htmlDoc.doc.Find("#allow-edits-from-maintainers").Attr("data-url")
		assert.True(t, exists)
		req := NewRequestWithValues(t, "POST", dataURL+"/set_allow_maintainer_edit", map[string]string{
			"allow_maintainer_edit": "true",
		})
		user4Session.MakeRequest(t, req, http.StatusOK)

		// user2 (admin of repo3) goes to the PR files page again, now the file can be edited
		resp = user2Session.MakeRequest(t, NewRequest(t, "GET", prURL+"/files"), http.StatusOK)
		editFileLink := diffViewerConfigOf(t, NewHTMLParser(t, resp.Body)).EditFileLink
		require.NotEmpty(t, editFileLink)
		editFileLink += "README.md"
		resp = user2Session.MakeRequest(t, NewRequest(t, "GET", editFileLink), http.StatusOK)
		lastCommit := NewHTMLParser(t, resp.Body).GetInputValueByName("last_commit")
		assert.NotEmpty(t, lastCommit)
		req = NewRequestWithValues(t, "POST", editFileLink, map[string]string{
			"last_commit":    lastCommit,
			"tree_path":      "README.md",
			"content":        "File is edited by the maintainer user2",
			"commit_summary": "user2 updated the file",
			"commit_choice":  "direct",
		})
		resp = user2Session.MakeRequest(t, req, http.StatusOK)
		assert.NotEmpty(t, test.RedirectURL(resp))
	})
}
