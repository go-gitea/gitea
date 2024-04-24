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
	assert.Greater(t, editButtonCount, 0, "Expected to find a button to edit a file in the PR diff view but there were none")

	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		defer tests.PrepareTestEnv(t)()

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
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
		assert.Greater(t, editButtonCount, 0, "Expected to find a button to edit a file in the PR diff view but there were none")

		repoForked := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user1", Name: "repo1"})
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// delete the head repository and revisit the PR diff view
		err := repo_service.DeleteRepositoryDirectly(db.DefaultContext, user2, repoForked.ID)
		assert.NoError(t, err)

		req = NewRequest(t, "GET", prFilesURL)
		resp = session.MakeRequest(t, req, http.StatusOK)
		doc = NewHTMLParser(t, resp.Body)
		editButtonCount = doc.doc.Find(".diff-file-header-actions a[href*='/_edit/']").Length()
		assert.EqualValues(t, editButtonCount, 0, "Expected not to find a button to edit a file in the PR diff view because head repository has been deleted")
	})
}
