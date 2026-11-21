// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	issue_service "gitea.dev/services/issue"
	pull_service "gitea.dev/services/pull"
	repo_service "gitea.dev/services/repository"
	files_service "gitea.dev/services/repository/files"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullReview(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	t.Run("ReviewerMissed", testPullReviewReviewerMissed)
	t.Run("LockedOrBlocked", testPullReviewLockedOrBlocked)
}

func testPullReviewReviewerMissed(t *testing.T) {
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

func testPullReviewLockedOrBlocked(t *testing.T) {
	require.NoError(t, issues_model.UpdateIssueCols(t.Context(), &issues_model.Issue{ID: 3, IsLocked: true}, "is_locked"))
	for _, tc := range []struct {
		reviewerID, index, replyCommentID int64
		webStatus                         int
	}{
		{4, 3, 7, http.StatusBadRequest},
		{34, 2, 4, http.StatusForbidden},
	} {
		reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: tc.reviewerID})
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: 1, Index: tc.index})
		pendingReview, err := issues_model.CreateReview(t.Context(), issues_model.CreateReviewOptions{Type: issues_model.ReviewTypePending, Issue: issue, Reviewer: reviewer})
		require.NoError(t, err)
		commentCount := unittest.GetCount(t, &issues_model.Comment{IssueID: issue.ID})
		pullPath := fmt.Sprintf("user2/repo1/pulls/%d", tc.index)
		session := loginUser(t, reviewer.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		testSubmitReview(t, session, "user2", "repo1", strconv.FormatInt(tc.index, 10), "", "comment", tc.webStatus)
		session.MakeRequest(t, NewRequestWithValues(t, "POST", "/"+pullPath+"/files/reviews/comments", map[string]string{
			"origin": "diff", "content": "test", "path": "README.md", "side": "proposed", "line": "1",
		}), tc.webStatus)
		MakeRequest(t, NewRequestWithJSON(t, "POST", "/api/v1/repos/"+pullPath+"/reviews", &api.CreatePullReviewOptions{Event: api.ReviewStateComment, Body: "test"}).AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/reviews/%d", pullPath, pendingReview.ID), &api.SubmitPullReviewOptions{Event: api.ReviewStateComment, Body: "test"}).AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/comments/%d/replies", pullPath, tc.replyCommentID), &api.CreatePullReviewCommentReplyOptions{Body: "test"}).AddTokenAuth(token), http.StatusForbidden)
		unittest.AssertCount(t, &issues_model.Comment{IssueID: issue.ID}, commentCount)
	}
	MakeRequest(t, NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/pulls/3/reviews", &api.CreatePullReviewOptions{Event: api.ReviewStateComment, Body: "test"}).AddTokenAuth(getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)), http.StatusOK)

	for name, status := range map[string]int{"user5": http.StatusForbidden, "user40": http.StatusOK} {
		require.NoError(t, db.Insert(t.Context(), &user_model.Blocking{BlockerID: 1, BlockeeID: unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: name}).ID}))
		testSubmitReview(t, loginUser(t, name), "user2", "repo1", "2", "", "comment", status)
	}
}

func TestPullView_CodeOwner(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
		user8 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})

		// Create the repo.
		repo, err := repo_service.CreateRepositoryDirectly(t.Context(), user2, user2, repo_service.CreateRepoOptions{
			Name:             "test_codeowner",
			Readme:           "Default",
			AutoInit:         true,
			ObjectFormatName: git.Sha1ObjectFormat.Name(),
			DefaultBranch:    "master",
		}, true)
		assert.NoError(t, err)

		for _, reviewer := range []*user_model.User{user5, user8} {
			require.NoError(t, repo_model.WatchRepoWithOptions(t.Context(), reviewer, repo, repo_model.WatchOptions{
				Mode: repo_model.WatchModeCustom, WatchPullRequests: true,
			}))
		}

		// create code owner branch protection
		protectBranch := git_model.ProtectedBranch{
			BlockOnCodeownerReviews: true,
			RepoID:                  repo.ID,
			RuleName:                "master",
			CanPush:                 true,
		}

		err = pull_service.CreateOrUpdateProtectedBranch(t.Context(), repo, &protectBranch, git_model.WhitelistOptions{})
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

		t.Run("File owners", func(t *testing.T) {
			session := loginUser(t, "user2")
			resp := session.MakeRequest(t, NewRequest(t, "GET", "/user2/test_codeowner/src/branch/master/README.md"), http.StatusOK)
			doc := NewHTMLParser(t, resp.Body)
			assert.Equal(t, "@user5", doc.Find(".file-codeowners a").Text())

			_, err := files_service.ChangeRepoFiles(t.Context(), repo, user2, &files_service.ChangeRepoFilesOptions{
				OldBranch: repo.DefaultBranch,
				NewBranch: "file-owners",
				Files: []*files_service.ChangeRepoFile{{
					Operation:     "update",
					TreePath:      "CODEOWNERS",
					ContentReader: strings.NewReader("## review-request=false\n[Docs] @user8 @org3/team1\n*.md\n"),
				}},
			})
			require.NoError(t, err)
			resp = session.MakeRequest(t, NewRequest(t, "GET", "/user2/test_codeowner/src/branch/file-owners/README.md"), http.StatusOK)
			doc = NewHTMLParser(t, resp.Body)
			assert.Equal(t, "@user8", doc.Find(".file-codeowners a[href='/user8']").Text())
			assert.Equal(t, "@org3/team1", doc.Find(".file-codeowners a[href='/org3/teams/team1']").Text())
			resp = session.MakeRequest(t, NewRequest(t, "GET", "/user2/test_codeowner/src/branch/file-owners/CODEOWNERS"), http.StatusOK)
			doc = NewHTMLParser(t, resp.Body)
			assert.Zero(t, doc.Find(".file-codeowners").Length())
		})

		readmeBranchOptions := func(branch, content string) *files_service.ChangeRepoFilesOptions {
			return &files_service.ChangeRepoFilesOptions{
				NewBranch: branch,
				Files: []*files_service.ChangeRepoFile{{
					Operation:     "update",
					TreePath:      "README.md",
					ContentReader: strings.NewReader(content),
				}},
			}
		}

		t.Run("First Pull Request", func(t *testing.T) {
			// create a new branch to prepare for pull request
			_, err := files_service.ChangeRepoFiles(t.Context(), repo, user2, readmeBranchOptions("codeowner-basebranch", "# This is a new project\n"))
			assert.NoError(t, err)

			// Create a pull request.
			session := loginUser(t, "user2")
			testPullCreate(t, session, "user2", "test_codeowner", false, repo.DefaultBranch, "codeowner-basebranch", "Test Pull Request")

			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, HeadRepoID: repo.ID, HeadBranch: "codeowner-basebranch"})
			unittest.AssertExistsAndLoadBean(t, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest, ReviewerID: 5})
			assert.NoError(t, pr.LoadIssue(t.Context()))

			// update the file on the pr branch
			resp, err := files_service.ChangeRepoFiles(t.Context(), repo, user2, &files_service.ChangeRepoFilesOptions{
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

			// the push above requests user8's review asynchronously
			require.Eventually(t, func() bool {
				return unittest.GetCount(t, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest, ReviewerID: 8}) == 1
			}, 10*time.Second, 20*time.Millisecond, "code owner user8 was never requested for review")
			unittest.AssertCount(t, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest}, 2) // only user5 and user8

			prUpdated := pr
			for _, title := range []string{"[WIP] Test Pull Request", "Test Pull Request2"} {
				err = issue_service.ChangeTitle(t.Context(), prUpdated.Issue, user2, title)
				assert.NoError(t, err)
				prUpdated = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
				assert.NoError(t, prUpdated.LoadIssue(t.Context()))
				assert.Equal(t, title, prUpdated.Issue.Title)
			}

			// ensure it cannot be merged
			hasCodeownerReviews := issue_service.HasAllRequiredCodeownerReviews(t.Context(), &protectBranch, pr)
			assert.False(t, hasCodeownerReviews)

			_, _, err = issues_model.SubmitReview(t.Context(), user5, pr.Issue, issues_model.ReviewTypeApprove, "Very good", resp.Commit.SHA, false, make([]string, 0))
			assert.NoError(t, err)

			// should still fail (we also need user8)
			hasCodeownerReviews = issue_service.HasAllRequiredCodeownerReviews(t.Context(), &protectBranch, pr)
			assert.False(t, hasCodeownerReviews)

			_, _, err = issues_model.SubmitReview(t.Context(), user8, pr.Issue, issues_model.ReviewTypeApprove, "Very good", resp.Commit.SHA, false, make([]string, 0))
			assert.NoError(t, err)

			// now we should be able to merge
			hasCodeownerReviews = issue_service.HasAllRequiredCodeownerReviews(t.Context(), &protectBranch, pr)
			assert.True(t, hasCodeownerReviews)

			_, err = issue_service.ReviewRequest(t.Context(), pr.Issue, user5, nil, user5, true)
			assert.NoError(t, err)
			hasCodeownerReviews = issue_service.HasAllRequiredCodeownerReviews(t.Context(), &protectBranch, pr)
			assert.False(t, hasCodeownerReviews)

			// a later requested-changes review from a required code owner must override
			// that user's earlier approval and no longer satisfy the requirement
			_, _, err = issues_model.SubmitReview(t.Context(), user5, pr.Issue, issues_model.ReviewTypeReject, "Needs changes", resp.Commit.SHA, false, make([]string, 0))
			assert.NoError(t, err)
			hasCodeownerReviews = issue_service.HasAllRequiredCodeownerReviews(t.Context(), &protectBranch, pr)
			assert.False(t, hasCodeownerReviews)
		})

		t.Run("Dismissed Code Owner Review", func(t *testing.T) {
			_, err := files_service.ChangeRepoFiles(t.Context(), repo, user2, readmeBranchOptions("codeowner-dismiss-branch", "# New README content\n"))
			assert.NoError(t, err)

			session := loginUser(t, "user2")
			testPullCreate(t, session, "user2", "test_codeowner", false, repo.DefaultBranch, "codeowner-dismiss-branch", "Test Code Owner Dismissal")

			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, HeadBranch: "codeowner-dismiss-branch"})
			unittest.AssertExistsAndLoadBean(t, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest, ReviewerID: 5})

			hasCodeownerReviews := issue_service.HasAllRequiredCodeownerReviews(t.Context(), &protectBranch, pr)
			assert.False(t, hasCodeownerReviews)

			_, _, err = issues_model.SubmitReview(t.Context(), user5, pr.Issue, issues_model.ReviewTypeApprove, " LGTM", "", false, make([]string, 0))
			assert.NoError(t, err)

			hasCodeownerReviews = issue_service.HasAllRequiredCodeownerReviews(t.Context(), &protectBranch, pr)
			assert.True(t, hasCodeownerReviews)

			review := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{IssueID: pr.IssueID, ReviewerID: 5, Type: issues_model.ReviewTypeApprove})
			assert.False(t, review.Dismissed)

			err = issues_model.DismissReview(t.Context(), review, true)
			assert.NoError(t, err)

			review, err = issues_model.GetReviewByID(t.Context(), review.ID)
			assert.NoError(t, err)
			assert.True(t, review.Dismissed)

			hasCodeownerReviews = issue_service.HasAllRequiredCodeownerReviews(t.Context(), &protectBranch, pr)
			assert.False(t, hasCodeownerReviews)
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
			resp, err := files_service.ChangeRepoFiles(t.Context(), repo, user2, readmeBranchOptions("codeowner-basebranch2", "# This is a new project2\n"))
			assert.NoError(t, err)

			// Create a pull request.
			session := loginUser(t, "user2")
			testPullCreate(t, session, "user2", "test_codeowner", false, repo.DefaultBranch, "codeowner-basebranch2", "Test Pull Request2")

			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, HeadBranch: "codeowner-basebranch2"})
			unittest.AssertExistsAndLoadBean(t, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest, ReviewerID: 8})

			// should need user8 approval only now
			hasCodeownerReviews := issue_service.HasAllRequiredCodeownerReviews(t.Context(), &protectBranch, pr)
			assert.False(t, hasCodeownerReviews)

			// exercise the real merge-time gate, not just the helper: the persisted
			// protection rule must block the merge on the missing code owner review
			err = pull_service.CheckPullBranchProtections(t.Context(), pr, false)
			assert.ErrorContains(t, err, "code owner")

			_, _, err = issues_model.SubmitReview(t.Context(), user8, pr.Issue, issues_model.ReviewTypeApprove, "Very good", resp.Commit.SHA, false, make([]string, 0))
			assert.NoError(t, err)

			hasCodeownerReviews = issue_service.HasAllRequiredCodeownerReviews(t.Context(), &protectBranch, pr)
			assert.True(t, hasCodeownerReviews)

			// and once the code owner has approved, the merge gate no longer blocks
			assert.NoError(t, pull_service.CheckPullBranchProtections(t.Context(), pr, false))
		})

		t.Run("Forked Repo Pull Request", func(t *testing.T) {
			user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
			forkedRepo, err := repo_service.ForkRepository(t.Context(), user2, user5, repo_service.ForkRepoOptions{
				BaseRepo: repo,
				Name:     "test_codeowner",
			})
			assert.NoError(t, err)

			// create a new branch to prepare for pull request
			resp, err := files_service.ChangeRepoFiles(t.Context(), forkedRepo, user5, readmeBranchOptions("codeowner-basebranch-forked", "# This is a new forked project\n"))
			assert.NoError(t, err)

			session := loginUser(t, "user5")

			// create a pull request on the forked repository, code reviewers should not be mentioned
			testPullCreateDirectly(t, session, createPullRequestOptions{
				BaseRepoOwner: "user5",
				BaseRepoName:  "test_codeowner",
				BaseBranch:    forkedRepo.DefaultBranch,
				HeadRepoOwner: "",
				HeadRepoName:  "",
				HeadBranch:    "codeowner-basebranch-forked",
				Title:         "Test Pull Request on Forked Repository",
			})

			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: forkedRepo.ID, HeadBranch: "codeowner-basebranch-forked"})
			unittest.AssertNotExistsBean(t, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest, ReviewerID: 8})

			// create a pull request to base repository, code reviewers should be mentioned
			testPullCreateDirectly(t, session, createPullRequestOptions{
				BaseRepoOwner: repo.OwnerName,
				BaseRepoName:  repo.Name,
				BaseBranch:    repo.DefaultBranch,
				HeadRepoOwner: forkedRepo.OwnerName,
				HeadRepoName:  forkedRepo.Name,
				HeadBranch:    "codeowner-basebranch-forked",
				Title:         "Test Pull Request3",
			})

			pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, HeadRepoID: forkedRepo.ID, HeadBranch: "codeowner-basebranch-forked"})
			unittest.AssertExistsAndLoadBean(t, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest, ReviewerID: 8})

			// will also need user8 for this
			hasCodeownerReviews := issue_service.HasAllRequiredCodeownerReviews(t.Context(), &protectBranch, pr)
			assert.False(t, hasCodeownerReviews)

			_, _, err = issues_model.SubmitReview(t.Context(), user5, pr.Issue, issues_model.ReviewTypeApprove, "Very good", resp.Commit.SHA, false, make([]string, 0))
			assert.NoError(t, err)

			// should still fail (user5 is not a code owner for this PR)
			hasCodeownerReviews = issue_service.HasAllRequiredCodeownerReviews(t.Context(), &protectBranch, pr)
			assert.False(t, hasCodeownerReviews)

			_, _, err = issues_model.SubmitReview(t.Context(), user8, pr.Issue, issues_model.ReviewTypeApprove, "Very good", resp.Commit.SHA, false, make([]string, 0))
			assert.NoError(t, err)

			hasCodeownerReviews = issue_service.HasAllRequiredCodeownerReviews(t.Context(), &protectBranch, pr)
			assert.True(t, hasCodeownerReviews)
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

			// Submit an approve review on the PR.
			testSubmitReview(t, user2Session, "user2", "repo1", elem[4], "", "approve", http.StatusUnprocessableEntity)

			// Submit a reject review on the PR.
			testSubmitReview(t, user2Session, "user2", "repo1", elem[4], "", "reject", http.StatusUnprocessableEntity)
		})

		t.Run("Submit approve/reject review on closed PR", func(t *testing.T) {
			// Created a closed PR (made by user1) in the upstream repo1.
			testEditFileToNewBranch(t, user1Session, "user1", "repo1", "master", "a-test-branch", "README.md", "Hello, World (Edited...again)\n")
			resp := testPullCreate(t, user1Session, "user1", "repo1", false, "master", "a-test-branch", "This is a pull title")
			elem := strings.Split(test.RedirectURL(resp), "/")
			assert.Equal(t, "pulls", elem[3])
			testIssueClose(t, user1Session, elem[1], elem[2], elem[4])

			// Submit an approve review on the PR.
			testSubmitReview(t, user2Session, "user2", "repo1", elem[4], "", "approve", http.StatusUnprocessableEntity)

			// Submit a reject review on the PR.
			testSubmitReview(t, user2Session, "user2", "repo1", elem[4], "", "reject", http.StatusUnprocessableEntity)
		})
	})
}

func testSubmitReview(t *testing.T, session *TestSession, owner, repo, pullNumber, commitID, reviewType string, expectedSubmitStatus int) *httptest.ResponseRecorder {
	options := map[string]string{
		"commit_id": commitID,
		"content":   "test",
		"type":      reviewType,
	}

	submitURL := "/" + path.Join(owner, repo, "pulls", pullNumber, "files", "reviews", "submit")
	req := NewRequestWithValues(t, "POST", submitURL, options)
	return session.MakeRequest(t, req, expectedSubmitStatus)
}

func testIssueClose(t *testing.T, session *TestSession, owner, repo, issueNumber string) *httptest.ResponseRecorder {
	closeURL := "/" + path.Join(owner, repo, "issues", issueNumber, "comments")

	options := map[string]string{
		"status": "close",
	}

	req := NewRequestWithValues(t, "POST", closeURL, options)
	return session.MakeRequest(t, req, http.StatusOK)
}
