// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/test"
	issue_service "code.gitea.io/gitea/services/issue"
	repo_service "code.gitea.io/gitea/services/repository"
	files_service "code.gitea.io/gitea/services/repository/files"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestPullView_ReviewerMissed(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user1")

	req := NewRequest(t, "GET", "/pulls")
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.True(t, test.IsNormalPageCompleted(resp.Body.String()))

	req = NewRequest(t, "GET", "/user2/repo1/pulls/3")
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.True(t, test.IsNormalPageCompleted(resp.Body.String()))

	// if some reviews are missing, the page shouldn't fail
	err := db.TruncateBeans(db.DefaultContext, &issues_model.Review{})
	assert.NoError(t, err)
	req = NewRequest(t, "GET", "/user2/repo1/pulls/2")
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.True(t, test.IsNormalPageCompleted(resp.Body.String()))
}

func TestPullView_CodeOwner(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// Create the repo.
		repo, err := repo_service.CreateRepositoryDirectly(db.DefaultContext, user2, user2, repo_service.CreateRepoOptions{
			Name:             "test_codeowner",
			Readme:           "Default",
			AutoInit:         true,
			ObjectFormatName: git.Sha1ObjectFormat.Name(),
			DefaultBranch:    "master",
		})
		assert.NoError(t, err)

		// add CODEOWNERS to default branch
		_, err = files_service.ChangeRepoFiles(db.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			OldBranch: repo.DefaultBranch,
			Files: []*files_service.ChangeRepoFile{
				{
					Operation:     "create",
					TreePath:      "CODEOWNERS",
					ContentReader: strings.NewReader("README.md @user5\n"),
				},
			},
		})
		assert.NoError(t, err)

		t.Run("First Pull Request", func(t *testing.T) {
			// create a new branch to prepare for pull request
			_, err = files_service.ChangeRepoFiles(db.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
				NewBranch: "codeowner-basebranch",
				Files: []*files_service.ChangeRepoFile{
					{
						Operation:     "update",
						TreePath:      "README.md",
						ContentReader: strings.NewReader("# This is a new project\n"),
					},
				},
			})
			assert.NoError(t, err)

			// Create a pull request.
			session := loginUser(t, "user2")
			testPullCreate(t, session, "user2", "test_codeowner", false, repo.DefaultBranch, "codeowner-basebranch", "Test Pull Request")

			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, HeadRepoID: repo.ID, HeadBranch: "codeowner-basebranch"})
			unittest.AssertExistsIf(t, true, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest, ReviewerID: 5})
			assert.NoError(t, pr.LoadIssue(db.DefaultContext))

			err := issue_service.ChangeTitle(db.DefaultContext, pr.Issue, user2, "[WIP] Test Pull Request")
			assert.NoError(t, err)
			prUpdated1 := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
			assert.NoError(t, prUpdated1.LoadIssue(db.DefaultContext))
			assert.EqualValues(t, "[WIP] Test Pull Request", prUpdated1.Issue.Title)

			err = issue_service.ChangeTitle(db.DefaultContext, prUpdated1.Issue, user2, "Test Pull Request2")
			assert.NoError(t, err)
			prUpdated2 := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
			assert.NoError(t, prUpdated2.LoadIssue(db.DefaultContext))
			assert.EqualValues(t, "Test Pull Request2", prUpdated2.Issue.Title)
		})

		// change the default branch CODEOWNERS file to change README.md's codeowner
		_, err = files_service.ChangeRepoFiles(db.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation:     "update",
					TreePath:      "CODEOWNERS",
					ContentReader: strings.NewReader("README.md @user8\n"),
				},
			},
		})
		assert.NoError(t, err)

		t.Run("Second Pull Request", func(t *testing.T) {
			// create a new branch to prepare for pull request
			_, err = files_service.ChangeRepoFiles(db.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
				NewBranch: "codeowner-basebranch2",
				Files: []*files_service.ChangeRepoFile{
					{
						Operation:     "update",
						TreePath:      "README.md",
						ContentReader: strings.NewReader("# This is a new project2\n"),
					},
				},
			})
			assert.NoError(t, err)

			// Create a pull request.
			session := loginUser(t, "user2")
			testPullCreate(t, session, "user2", "test_codeowner", false, repo.DefaultBranch, "codeowner-basebranch2", "Test Pull Request2")

			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, HeadBranch: "codeowner-basebranch2"})
			unittest.AssertExistsIf(t, true, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest, ReviewerID: 8})
		})

		t.Run("Forked Repo Pull Request", func(t *testing.T) {
			user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
			forkedRepo, err := repo_service.ForkRepository(db.DefaultContext, user2, user5, repo_service.ForkRepoOptions{
				BaseRepo: repo,
				Name:     "test_codeowner",
			})
			assert.NoError(t, err)

			// create a new branch to prepare for pull request
			_, err = files_service.ChangeRepoFiles(db.DefaultContext, forkedRepo, user5, &files_service.ChangeRepoFilesOptions{
				NewBranch: "codeowner-basebranch-forked",
				Files: []*files_service.ChangeRepoFile{
					{
						Operation:     "update",
						TreePath:      "README.md",
						ContentReader: strings.NewReader("# This is a new forked project\n"),
					},
				},
			})
			assert.NoError(t, err)

			session := loginUser(t, "user5")

			// create a pull request on the forked repository, code reviewers should not be mentioned
			testPullCreateDirectly(t, session, "user5", "test_codeowner", forkedRepo.DefaultBranch, "", "", "codeowner-basebranch-forked", "Test Pull Request on Forked Repository")

			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: forkedRepo.ID, HeadBranch: "codeowner-basebranch-forked"})
			unittest.AssertExistsIf(t, false, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest, ReviewerID: 8})

			// create a pull request to base repository, code reviewers should be mentioned
			testPullCreateDirectly(t, session, repo.OwnerName, repo.Name, repo.DefaultBranch, forkedRepo.OwnerName, forkedRepo.Name, "codeowner-basebranch-forked", "Test Pull Request3")

			pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, HeadRepoID: forkedRepo.ID, HeadBranch: "codeowner-basebranch-forked"})
			unittest.AssertExistsIf(t, true, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest, ReviewerID: 8})
		})
	})
}

func TestPullView_GivenApproveOrRejectReviewOnClosedPR(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		user1Session := loginUser(t, "user1")
		user2Session := loginUser(t, "user2")

		// Have user1 create a fork of repo1.
		testRepoFork(t, user1Session, "user2", "repo1", "user1", "repo1", "")

		t.Run("Submit approve/reject review on merged PR", func(t *testing.T) {
			// Create a merged PR (made by user1) in the upstream repo1.
			testEditFile(t, user1Session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")
			resp := testPullCreate(t, user1Session, "user1", "repo1", false, "master", "master", "This is a pull title")
			elem := strings.Split(test.RedirectURL(resp), "/")
			assert.EqualValues(t, "pulls", elem[3])
			testPullMerge(t, user1Session, elem[1], elem[2], elem[4], repo_model.MergeStyleMerge, false)

			// Grab the CSRF token.
			req := NewRequest(t, "GET", path.Join(elem[1], elem[2], "pulls", elem[4]))
			resp = user2Session.MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)

			// Submit an approve review on the PR.
			testSubmitReview(t, user2Session, htmlDoc.GetCSRF(), "user2", "repo1", elem[4], "", "approve", http.StatusUnprocessableEntity)

			// Submit a reject review on the PR.
			testSubmitReview(t, user2Session, htmlDoc.GetCSRF(), "user2", "repo1", elem[4], "", "reject", http.StatusUnprocessableEntity)
		})

		t.Run("Submit approve/reject review on closed PR", func(t *testing.T) {
			// Created a closed PR (made by user1) in the upstream repo1.
			testEditFileToNewBranch(t, user1Session, "user1", "repo1", "master", "a-test-branch", "README.md", "Hello, World (Editied...again)\n")
			resp := testPullCreate(t, user1Session, "user1", "repo1", false, "master", "a-test-branch", "This is a pull title")
			elem := strings.Split(test.RedirectURL(resp), "/")
			assert.EqualValues(t, "pulls", elem[3])
			testIssueClose(t, user1Session, elem[1], elem[2], elem[4])

			// Grab the CSRF token.
			req := NewRequest(t, "GET", path.Join(elem[1], elem[2], "pulls", elem[4]))
			resp = user2Session.MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)

			// Submit an approve review on the PR.
			testSubmitReview(t, user2Session, htmlDoc.GetCSRF(), "user2", "repo1", elem[4], "", "approve", http.StatusUnprocessableEntity)

			// Submit a reject review on the PR.
			testSubmitReview(t, user2Session, htmlDoc.GetCSRF(), "user2", "repo1", elem[4], "", "reject", http.StatusUnprocessableEntity)
		})
	})
}

func testSubmitReview(t *testing.T, session *TestSession, csrf, owner, repo, pullNumber, commitID, reviewType string, expectedSubmitStatus int) *httptest.ResponseRecorder {
	options := map[string]string{
		"_csrf":     csrf,
		"commit_id": commitID,
		"content":   "test",
		"type":      reviewType,
	}

	submitURL := path.Join(owner, repo, "pulls", pullNumber, "files", "reviews", "submit")
	req := NewRequestWithValues(t, "POST", submitURL, options)
	return session.MakeRequest(t, req, expectedSubmitStatus)
}

func testIssueClose(t *testing.T, session *TestSession, owner, repo, issueNumber string) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", path.Join(owner, repo, "pulls", issueNumber))
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	closeURL := path.Join(owner, repo, "issues", issueNumber, "comments")

	options := map[string]string{
		"_csrf":  htmlDoc.GetCSRF(),
		"status": "close",
	}

	req = NewRequestWithValues(t, "POST", closeURL, options)
	return session.MakeRequest(t, req, http.StatusOK)
}
