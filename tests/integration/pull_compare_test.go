// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/test"
	repo_service "code.gitea.io/gitea/services/repository"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestPullCompare(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/pulls")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find(".new-pr-button").Attr("href")
	assert.True(t, exists, "The template has changed")

	req = NewRequest(t, "GET", link)
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.EqualValues(t, http.StatusOK, resp.Code)

	// test the edit button in the PR diff view
	req = NewRequest(t, "GET", "/user2/repo1/pulls/3/files")
	resp = session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)
	editButtonCount := doc.doc.Find(".diff-file-header-actions a[href*='/_edit/']").Length()
	assert.Positive(t, editButtonCount, "Expected to find a button to edit a file in the PR diff view but there were none")

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
		req = NewRequest(t, "GET", prFilesURL)
		resp = session.MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)
		editButtonCount := doc.doc.Find(".diff-file-header-actions a[href*='/_edit/']").Length()
		assert.Positive(t, editButtonCount, "Expected to find a button to edit a file in the PR diff view but there were none")

		repoForked := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user1", Name: "repo1"})
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// delete the head repository and revisit the PR diff view
		err := repo_service.DeleteRepositoryDirectly(db.DefaultContext, user2, repoForked.ID)
		assert.NoError(t, err)

		req = NewRequest(t, "GET", prFilesURL)
		resp = session.MakeRequest(t, req, http.StatusOK)
		doc = NewHTMLParser(t, resp.Body)
		editButtonCount = doc.doc.Find(".diff-file-header-actions a[href*='/_edit/']").Length()
		assert.EqualValues(t, 0, editButtonCount, "Expected not to find a button to edit a file in the PR diff view because head repository has been deleted")
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
		resp := testPullCreateDirectly(t, user4Session, repo3.OwnerName, repo3.Name, "master", "user4", forkedRepoName, "user4/update-readme", "PR for user4 forked repo3")
		prURL := test.RedirectURL(resp)

		// user2 (admin of repo3) goes to the PR files page
		user2Session := loginUser(t, "user2")
		resp = user2Session.MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("%s/files", prURL)), http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		nodes := htmlDoc.doc.Find(".diff-file-box[data-new-filename=\"README.md\"] .diff-file-header-actions .tippy-target a")
		if assert.Equal(t, 1, nodes.Length()) {
			// there is only "View File" button, no "Edit File" button
			assert.Equal(t, "View File", nodes.First().Text())
			viewFileLink, exists := nodes.First().Attr("href")
			if assert.True(t, exists) {
				user2Session.MakeRequest(t, NewRequest(t, "GET", viewFileLink), http.StatusOK)
			}
		}

		// user4 goes to the PR page and enable "Allow maintainers to edit"
		resp = user4Session.MakeRequest(t, NewRequest(t, "GET", prURL), http.StatusOK)
		htmlDoc = NewHTMLParser(t, resp.Body)
		dataURL, exists := htmlDoc.doc.Find("#allow-edits-from-maintainers").Attr("data-url")
		assert.True(t, exists)
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("%s/set_allow_maintainer_edit", dataURL), map[string]string{
			"_csrf":                 htmlDoc.GetCSRF(),
			"allow_maintainer_edit": "true",
		})
		user4Session.MakeRequest(t, req, http.StatusOK)

		// user2 (admin of repo3) goes to the PR files page again
		resp = user2Session.MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("%s/files", prURL)), http.StatusOK)
		htmlDoc = NewHTMLParser(t, resp.Body)
		nodes = htmlDoc.doc.Find(".diff-file-box[data-new-filename=\"README.md\"] .diff-file-header-actions .tippy-target a")
		if assert.Equal(t, 2, nodes.Length()) {
			// there are "View File" button and "Edit File" button
			assert.Equal(t, "View File", nodes.First().Text())
			viewFileLink, exists := nodes.First().Attr("href")
			if assert.True(t, exists) {
				user2Session.MakeRequest(t, NewRequest(t, "GET", viewFileLink), http.StatusOK)
			}

			assert.Equal(t, "Edit File", nodes.Last().Text())
			editFileLink, exists := nodes.Last().Attr("href")
			if assert.True(t, exists) {
				// edit the file
				resp := user2Session.MakeRequest(t, NewRequest(t, "GET", editFileLink), http.StatusOK)
				htmlDoc := NewHTMLParser(t, resp.Body)
				lastCommit := htmlDoc.GetInputValueByName("last_commit")
				assert.NotEmpty(t, lastCommit)
				req := NewRequestWithValues(t, "POST", editFileLink, map[string]string{
					"_csrf":          htmlDoc.GetCSRF(),
					"last_commit":    lastCommit,
					"tree_path":      "README.md",
					"content":        "File is edited by the maintainer user2",
					"commit_summary": "user2 updated the file",
					"commit_choice":  "direct",
				})
				user2Session.MakeRequest(t, req, http.StatusSeeOther)
			}
		}
	})
}
