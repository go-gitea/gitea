// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
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
	"code.gitea.io/gitea/modules/gitrepo"
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
	err := db.TruncateBeans(t.Context(), &issues_model.Review{})
	assert.NoError(t, err)
	req = NewRequest(t, "GET", "/user2/repo1/pulls/2")
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.True(t, test.IsNormalPageCompleted(resp.Body.String()))
}

func TestPullView_CodeOwner(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// Create the repo.
		repo, err := repo_service.CreateRepositoryDirectly(t.Context(), user2, user2, repo_service.CreateRepoOptions{
			Name:             "test_codeowner",
			Readme:           "Default",
			AutoInit:         true,
			ObjectFormatName: git.Sha1ObjectFormat.Name(),
			DefaultBranch:    "master",
		}, true)
		assert.NoError(t, err)

		// add CODEOWNERS to default branch
		_, err = files_service.ChangeRepoFiles(t.Context(), repo, user2, &files_service.ChangeRepoFilesOptions{
			OldBranch: repo.DefaultBranch,
			Files: []*files_service.ChangeRepoFile{
				{
					Operation:     "create",
					TreePath:      "CODEOWNERS",
					ContentReader: strings.NewReader("README.md @user5\nuser8-file.md @user8\n"),
				},
			},
		})
		assert.NoError(t, err)

		t.Run("First Pull Request", func(t *testing.T) {
			// create a new branch to prepare for pull request
			_, err := files_service.ChangeRepoFiles(t.Context(), repo, user2, &files_service.ChangeRepoFilesOptions{
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
			unittest.AssertExistsAndLoadBean(t, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest, ReviewerID: 5})
			assert.NoError(t, pr.LoadIssue(t.Context()))

			// update the file on the pr branch
			_, err = files_service.ChangeRepoFiles(t.Context(), repo, user2, &files_service.ChangeRepoFilesOptions{
				OldBranch: "codeowner-basebranch",
				Files: []*files_service.ChangeRepoFile{
					{
						Operation:     "create",
						TreePath:      "user8-file.md",
						ContentReader: strings.NewReader("# This is a new project2\n"),
					},
				},
			})
			assert.NoError(t, err)

			reviewNotifiers, err := issue_service.PullRequestCodeOwnersReview(t.Context(), pr)
			assert.NoError(t, err)
			assert.Len(t, reviewNotifiers, 1)
			assert.EqualValues(t, 8, reviewNotifiers[0].Reviewer.ID)

			err = issue_service.ChangeTitle(t.Context(), pr.Issue, user2, "[WIP] Test Pull Request")
			assert.NoError(t, err)
			prUpdated1 := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
			assert.NoError(t, prUpdated1.LoadIssue(t.Context()))
			assert.Equal(t, "[WIP] Test Pull Request", prUpdated1.Issue.Title)

			err = issue_service.ChangeTitle(t.Context(), prUpdated1.Issue, user2, "Test Pull Request2")
			assert.NoError(t, err)
			prUpdated2 := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
			assert.NoError(t, prUpdated2.LoadIssue(t.Context()))
			assert.Equal(t, "Test Pull Request2", prUpdated2.Issue.Title)
		})

		// change the default branch CODEOWNERS file to change README.md's codeowner
		_, err = files_service.ChangeRepoFiles(t.Context(), repo, user2, &files_service.ChangeRepoFilesOptions{
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
			_, err = files_service.ChangeRepoFiles(t.Context(), repo, user2, &files_service.ChangeRepoFilesOptions{
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
			unittest.AssertExistsAndLoadBean(t, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest, ReviewerID: 8})
		})

		t.Run("Forked Repo Pull Request", func(t *testing.T) {
			user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
			forkedRepo, err := repo_service.ForkRepository(t.Context(), user2, user5, repo_service.ForkRepoOptions{
				BaseRepo: repo,
				Name:     "test_codeowner",
			})
			assert.NoError(t, err)

			// create a new branch to prepare for pull request
			_, err = files_service.ChangeRepoFiles(t.Context(), forkedRepo, user5, &files_service.ChangeRepoFilesOptions{
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
			unittest.AssertNotExistsBean(t, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest, ReviewerID: 8})

			// create a pull request to base repository, code reviewers should be mentioned
			testPullCreateDirectly(t, session, repo.OwnerName, repo.Name, repo.DefaultBranch, forkedRepo.OwnerName, forkedRepo.Name, "codeowner-basebranch-forked", "Test Pull Request3")

			pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, HeadRepoID: forkedRepo.ID, HeadBranch: "codeowner-basebranch-forked"})
			unittest.AssertExistsAndLoadBean(t, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest, ReviewerID: 8})
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
			assert.Equal(t, "pulls", elem[3])
			testPullMerge(t, user1Session, elem[1], elem[2], elem[4], MergeOptions{
				Style:        repo_model.MergeStyleMerge,
				DeleteBranch: false,
			})

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
			testEditFileToNewBranch(t, user1Session, "user1", "repo1", "master", "a-test-branch", "README.md", "Hello, World (Edited...again)\n")
			resp := testPullCreate(t, user1Session, "user1", "repo1", false, "master", "a-test-branch", "This is a pull title")
			elem := strings.Split(test.RedirectURL(resp), "/")
			assert.Equal(t, "pulls", elem[3])
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

func Test_ReviewCodeComment(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// Create the repo.
		repo, err := repo_service.CreateRepositoryDirectly(t.Context(), user2, user2, repo_service.CreateRepoOptions{
			Name:             "test_codecomment",
			Readme:           "Default",
			AutoInit:         true,
			ObjectFormatName: git.Sha1ObjectFormat.Name(),
			DefaultBranch:    "master",
		}, true)
		assert.NoError(t, err)

		// add README.md to default branch
		_, err = files_service.ChangeRepoFiles(t.Context(), repo, user2, &files_service.ChangeRepoFilesOptions{
			OldBranch: repo.DefaultBranch,
			Files: []*files_service.ChangeRepoFile{
				{
					Operation:     "update",
					TreePath:      "README.md",
					ContentReader: strings.NewReader("# 111\n# 222\n"),
				},
			},
		})
		assert.NoError(t, err)

		var pr *issues_model.PullRequest
		t.Run("Create Pull Request", func(t *testing.T) {
			// create a new branch to prepare for pull request
			_, err = files_service.ChangeRepoFiles(t.Context(), repo, user2, &files_service.ChangeRepoFilesOptions{
				NewBranch: "branch_codecomment1",
				Files: []*files_service.ChangeRepoFile{
					{
						Operation: "update",
						TreePath:  "README.md",
						// add 5 lines to the file
						ContentReader: strings.NewReader("# 111\n# 222\n# 333\n# 444\n# 555\n# 666\n# 777\n"),
					},
				},
			})
			assert.NoError(t, err)

			// Create a pull request.
			session := loginUser(t, "user2")
			testPullCreate(t, session, "user2", "test_codecomment", false, repo.DefaultBranch, "branch_codecomment1", "Test Pull Request1")

			pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, HeadBranch: "branch_codecomment1"})
		})

		t.Run("Create Code Comment", func(t *testing.T) {
			session := loginUser(t, "user2")

			// Grab the CSRF token.
			req := NewRequest(t, "GET", path.Join("user2", "test_codecomment", "pulls", "1"))
			resp := session.MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)

			// Create a code comment on the pull request.
			commentURL := fmt.Sprintf("/user2/test_codecomment/pulls/%d/files/reviews/comments", pr.Index)
			options := map[string]string{
				"_csrf":            htmlDoc.GetCSRF(),
				"origin":           "diff",
				"content":          "code comment on right line 4",
				"side":             "proposed",
				"line":             "4",
				"path":             "README.md",
				"single_review":    "true",
				"reply":            "0",
				"before_commit_id": "",
				"after_commit_id":  "",
			}
			req = NewRequestWithValues(t, "POST", commentURL, options)
			session.MakeRequest(t, req, http.StatusOK)

			// Check if the comment was created.
			comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
				Type:    issues_model.CommentTypeCode,
				IssueID: pr.IssueID,
			})
			assert.Equal(t, "code comment on right line 4", comment.Content)
			assert.Equal(t, "README.md", comment.TreePath)
			assert.Equal(t, int64(4), comment.Line)
			gitRepo, err := gitrepo.OpenRepository(t.Context(), repo)
			assert.NoError(t, err)
			defer gitRepo.Close()
			commitID, err := gitRepo.GetRefCommitID(pr.GetGitHeadRefName())
			assert.NoError(t, err)
			assert.Equal(t, commitID, comment.CommitSHA)

			// load the files page and confirm the comment is there
			filesPageURL := fmt.Sprintf("/user2/test_codecomment/pulls/%d/files", pr.Index)
			req = NewRequest(t, "GET", filesPageURL)
			resp = session.MakeRequest(t, req, http.StatusOK)
			htmlDoc = NewHTMLParser(t, resp.Body)
			commentHTML := htmlDoc.Find(fmt.Sprintf("#issuecomment-%d", comment.ID))
			assert.NotNil(t, commentHTML)
			assert.Equal(t, "code comment on right line 4", strings.TrimSpace(commentHTML.Find(".comment-body .render-content").Text()))

			// the last line of this comment line number is 4
			parentTr := commentHTML.ParentsFiltered("tr").First()
			assert.NotNil(t, parentTr)
			previousTr := parentTr.PrevAllFiltered("tr").First()
			val, _ := previousTr.Attr("data-line-type")
			assert.Equal(t, "add", val)
			td := previousTr.Find("td.lines-num-new")
			val, _ = td.Attr("data-line-num")
			assert.Equal(t, "4", val)
		})

		t.Run("Pushing new commit to the pull request to add lines", func(t *testing.T) {
			// create a new branch to prepare for pull request
			_, err = files_service.ChangeRepoFiles(t.Context(), repo, user2, &files_service.ChangeRepoFilesOptions{
				OldBranch: "branch_codecomment1",
				Files: []*files_service.ChangeRepoFile{
					{
						Operation: "update",
						TreePath:  "README.md",
						// add 1 line before the code comment line 4
						ContentReader: strings.NewReader("# 111\n# 222\n# 333\n# 334\n# 444\n# 555\n# 666\n# 777\n"),
					},
				},
			})
			assert.NoError(t, err)

			session := loginUser(t, "user2")
			comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
				Type:    issues_model.CommentTypeCode,
				IssueID: pr.IssueID,
			})

			// load the files page and confirm the comment's line number is dynamically adjusted
			filesPageURL := fmt.Sprintf("/user2/test_codecomment/pulls/%d/files", pr.Index)
			req := NewRequest(t, "GET", filesPageURL)
			resp := session.MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)
			commentHTML := htmlDoc.Find(fmt.Sprintf("#issuecomment-%d", comment.ID))
			assert.NotNil(t, commentHTML)
			assert.Equal(t, "code comment on right line 4", strings.TrimSpace(commentHTML.Find(".comment-body .render-content").Text()))

			// the last line of this comment line number is 4
			parentTr := commentHTML.ParentsFiltered("tr").First()
			assert.NotNil(t, parentTr)
			previousTr := parentTr.PrevAllFiltered("tr").First()
			val, _ := previousTr.Attr("data-line-type")
			assert.Equal(t, "add", val)
			td := previousTr.Find("td.lines-num-new")
			val, _ = td.Attr("data-line-num")
			assert.Equal(t, "5", val) // one line have inserted in this commit, so the line number should be 5 now
		})

		t.Run("Pushing new commit to the pull request to delete lines", func(t *testing.T) {
			// create a new branch to prepare for pull request
			_, err := files_service.ChangeRepoFiles(t.Context(), repo, user2, &files_service.ChangeRepoFilesOptions{
				OldBranch: "branch_codecomment1",
				Files: []*files_service.ChangeRepoFile{
					{
						Operation: "update",
						TreePath:  "README.md",
						// delete the second line before the code comment line 4
						ContentReader: strings.NewReader("# 111\n# 333\n# 334\n# 444\n# 555\n# 666\n# 777\n"),
					},
				},
			})
			assert.NoError(t, err)

			session := loginUser(t, "user2")
			comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
				Type:    issues_model.CommentTypeCode,
				IssueID: pr.IssueID,
			})

			// load the files page and confirm the comment's line number is dynamically adjusted
			filesPageURL := fmt.Sprintf("/user2/test_codecomment/pulls/%d/files", pr.Index)
			req := NewRequest(t, "GET", filesPageURL)
			resp := session.MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)
			commentHTML := htmlDoc.Find(fmt.Sprintf("#issuecomment-%d", comment.ID))
			assert.NotNil(t, commentHTML)
			assert.Equal(t, "code comment on right line 4", strings.TrimSpace(commentHTML.Find(".comment-body .render-content").Text()))

			// the last line of this comment line number is 4
			parentTr := commentHTML.ParentsFiltered("tr").First()
			assert.NotNil(t, parentTr)
			previousTr := parentTr.PrevAllFiltered("tr").First()
			val, _ := previousTr.Attr("data-line-type")
			assert.Equal(t, "add", val)
			td := previousTr.Find("td.lines-num-new")
			val, _ = td.Attr("data-line-num")
			assert.Equal(t, "4", val) // one line have inserted and one line deleted before this line in this commit, so the line number should be 4 now

			// add a new comment on the deleted line
			commentURL := fmt.Sprintf("/user2/test_codecomment/pulls/%d/files/reviews/comments", pr.Index)
			options := map[string]string{
				"_csrf":            htmlDoc.GetCSRF(),
				"origin":           "diff",
				"content":          "code comment on left line 2",
				"side":             "previous",
				"line":             "2",
				"path":             "README.md",
				"single_review":    "true",
				"reply":            "0",
				"before_commit_id": "",
				"after_commit_id":  "",
			}
			req = NewRequestWithValues(t, "POST", commentURL, options)
			session.MakeRequest(t, req, http.StatusOK)
			// Check if the comment was created.
			commentLast := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
				Type:    issues_model.CommentTypeCode,
				IssueID: pr.IssueID,
				Content: "code comment on left line 2",
			})
			assert.Equal(t, "code comment on left line 2", commentLast.Content)
			assert.Equal(t, "README.md", commentLast.TreePath)
			assert.Equal(t, int64(-2), commentLast.Line)
			assert.Equal(t, pr.MergeBase, commentLast.BeforeCommitID)

			// load the files page and confirm the comment's line number is dynamically adjusted
			filesPageURL = fmt.Sprintf("/user2/test_codecomment/pulls/%d/files", pr.Index)
			req = NewRequest(t, "GET", filesPageURL)
			resp = session.MakeRequest(t, req, http.StatusOK)
			htmlDoc = NewHTMLParser(t, resp.Body)
			commentHTML = htmlDoc.Find(fmt.Sprintf("#issuecomment-%d", commentLast.ID))
			assert.NotNil(t, commentHTML)
			assert.Equal(t, "code comment on left line 2", strings.TrimSpace(commentHTML.Find(".comment-body .render-content").Text()))

			// the last line of this comment line number is 4
			parentTr = commentHTML.ParentsFiltered("tr").First()
			assert.NotNil(t, parentTr)
			previousTr = parentTr.PrevAllFiltered("tr").First()
			val, _ = previousTr.Attr("data-line-type")
			assert.Equal(t, "del", val)
			td = previousTr.Find("td.lines-num-old")
			val, _ = td.Attr("data-line-num")
			assert.Equal(t, "2", val)
		})
	})
}
