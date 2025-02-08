// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	issue_service "code.gitea.io/gitea/services/issue"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

func TestAPIPullReview(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	pullIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})
	assert.NoError(t, pullIssue.LoadAttributes(db.DefaultContext))
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pullIssue.RepoID})

	// test ListPullReviews
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews", repo.OwnerName, repo.Name, pullIssue.Index).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var reviews []*api.PullReview
	DecodeJSON(t, resp, &reviews)
	require.Len(t, reviews, 8)

	for _, r := range reviews {
		assert.EqualValues(t, pullIssue.HTMLURL(), r.HTMLPullURL)
	}
	assert.EqualValues(t, 8, reviews[3].ID)
	assert.EqualValues(t, "APPROVED", reviews[3].State)
	assert.EqualValues(t, 0, reviews[3].CodeCommentsCount)
	assert.True(t, reviews[3].Stale)
	assert.False(t, reviews[3].Official)

	assert.EqualValues(t, 10, reviews[5].ID)
	assert.EqualValues(t, "REQUEST_CHANGES", reviews[5].State)
	assert.EqualValues(t, 1, reviews[5].CodeCommentsCount)
	assert.EqualValues(t, -1, reviews[5].Reviewer.ID) // ghost user
	assert.False(t, reviews[5].Stale)
	assert.True(t, reviews[5].Official)

	// test GetPullReview
	req = NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews/%d", repo.OwnerName, repo.Name, pullIssue.Index, reviews[3].ID).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	var review api.PullReview
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, *reviews[3], review)

	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls/%d/reviews/%d", repo.OwnerName, repo.Name, pullIssue.Index, reviews[5].ID).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, *reviews[5], review)

	// test GetPullReviewComments
	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 7})
	req = NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews/%d/comments", repo.OwnerName, repo.Name, pullIssue.Index, 10).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	var reviewComments []*api.PullReviewComment
	DecodeJSON(t, resp, &reviewComments)
	assert.Len(t, reviewComments, 1)
	assert.EqualValues(t, "Ghost", reviewComments[0].Poster.UserName)
	assert.EqualValues(t, "a review from a deleted user", reviewComments[0].Body)
	assert.EqualValues(t, comment.ID, reviewComments[0].ID)
	assert.EqualValues(t, comment.UpdatedUnix, reviewComments[0].Updated.Unix())
	assert.EqualValues(t, comment.HTMLURL(db.DefaultContext), reviewComments[0].HTMLURL)

	// test CreatePullReview
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews", repo.OwnerName, repo.Name, pullIssue.Index), &api.CreatePullReviewOptions{
		Body: "body1",
		// Event: "" # will result in PENDING
		Comments: []api.CreatePullReviewComment{
			{
				Path:       "README.md",
				Body:       "first new line",
				OldLineNum: 0,
				NewLineNum: 1,
			}, {
				Path:       "README.md",
				Body:       "first old line",
				OldLineNum: 1,
				NewLineNum: 0,
			}, {
				Path:       "iso-8859-1.txt",
				Body:       "this line contains a non-utf-8 character",
				OldLineNum: 0,
				NewLineNum: 1,
			},
		},
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, 6, review.ID)
	assert.EqualValues(t, "PENDING", review.State)
	assert.EqualValues(t, 3, review.CodeCommentsCount)

	// test SubmitPullReview
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews/%d", repo.OwnerName, repo.Name, pullIssue.Index, review.ID), &api.SubmitPullReviewOptions{
		Event: "APPROVED",
		Body:  "just two nits",
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, 6, review.ID)
	assert.EqualValues(t, "APPROVED", review.State)
	assert.EqualValues(t, 3, review.CodeCommentsCount)

	// test dismiss review
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews/%d/dismissals", repo.OwnerName, repo.Name, pullIssue.Index, review.ID), &api.DismissPullReviewOptions{
		Message: "test",
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, 6, review.ID)
	assert.True(t, review.Dismissed)

	// test dismiss review
	req = NewRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews/%d/undismissals", repo.OwnerName, repo.Name, pullIssue.Index, review.ID)).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, 6, review.ID)
	assert.False(t, review.Dismissed)

	// test DeletePullReview
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews", repo.OwnerName, repo.Name, pullIssue.Index), &api.CreatePullReviewOptions{
		Body:  "just a comment",
		Event: "COMMENT",
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, "COMMENT", review.State)
	assert.EqualValues(t, 0, review.CodeCommentsCount)
	req = NewRequestf(t, http.MethodDelete, "/api/v1/repos/%s/%s/pulls/%d/reviews/%d", repo.OwnerName, repo.Name, pullIssue.Index, review.ID).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// test CreatePullReview Comment without body but with comments
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews", repo.OwnerName, repo.Name, pullIssue.Index), &api.CreatePullReviewOptions{
		// Body:  "",
		Event: "COMMENT",
		Comments: []api.CreatePullReviewComment{
			{
				Path:       "README.md",
				Body:       "first new line",
				OldLineNum: 0,
				NewLineNum: 1,
			}, {
				Path:       "README.md",
				Body:       "first old line",
				OldLineNum: 1,
				NewLineNum: 0,
			},
		},
	}).AddTokenAuth(token)
	var commentReview api.PullReview

	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &commentReview)
	assert.EqualValues(t, "COMMENT", commentReview.State)
	assert.EqualValues(t, 2, commentReview.CodeCommentsCount)
	assert.Empty(t, commentReview.Body)
	assert.False(t, commentReview.Dismissed)

	// test CreatePullReview Comment with body but without comments
	commentBody := "This is a body of the comment."
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews", repo.OwnerName, repo.Name, pullIssue.Index), &api.CreatePullReviewOptions{
		Body:     commentBody,
		Event:    "COMMENT",
		Comments: []api.CreatePullReviewComment{},
	}).AddTokenAuth(token)

	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &commentReview)
	assert.EqualValues(t, "COMMENT", commentReview.State)
	assert.EqualValues(t, 0, commentReview.CodeCommentsCount)
	assert.EqualValues(t, commentBody, commentReview.Body)
	assert.False(t, commentReview.Dismissed)

	// test CreatePullReview Comment without body and no comments
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews", repo.OwnerName, repo.Name, pullIssue.Index), &api.CreatePullReviewOptions{
		Body:     "",
		Event:    "COMMENT",
		Comments: []api.CreatePullReviewComment{},
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusUnprocessableEntity)
	errMap := make(map[string]any)
	json.Unmarshal(resp.Body.Bytes(), &errMap)
	assert.EqualValues(t, "review event COMMENT requires a body or a comment", errMap["message"].(string))

	// test get review requests
	// to make it simple, use same api with get review
	pullIssue12 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 12})
	assert.NoError(t, pullIssue12.LoadAttributes(db.DefaultContext))
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pullIssue12.RepoID})

	req = NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews", repo3.OwnerName, repo3.Name, pullIssue12.Index).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &reviews)
	assert.EqualValues(t, 11, reviews[0].ID)
	assert.EqualValues(t, "REQUEST_REVIEW", reviews[0].State)
	assert.EqualValues(t, 0, reviews[0].CodeCommentsCount)
	assert.False(t, reviews[0].Stale)
	assert.True(t, reviews[0].Official)
	assert.EqualValues(t, "test_team", reviews[0].ReviewerTeam.Name)

	assert.EqualValues(t, 12, reviews[1].ID)
	assert.EqualValues(t, "REQUEST_REVIEW", reviews[1].State)
	assert.EqualValues(t, 0, reviews[0].CodeCommentsCount)
	assert.False(t, reviews[1].Stale)
	assert.True(t, reviews[1].Official)
	assert.EqualValues(t, 1, reviews[1].Reviewer.ID)
}

func TestAPIPullReviewRequest(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	pullIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})
	assert.NoError(t, pullIssue.LoadAttributes(db.DefaultContext))
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pullIssue.RepoID})

	// Test add Review Request
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", repo.OwnerName, repo.Name, pullIssue.Index), &api.PullReviewRequestOptions{
		Reviewers: []string{"user4@example.com", "user8"},
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)

	// poster of pr can't be reviewer
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", repo.OwnerName, repo.Name, pullIssue.Index), &api.PullReviewRequestOptions{
		Reviewers: []string{"user1"},
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	// test user not exist
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", repo.OwnerName, repo.Name, pullIssue.Index), &api.PullReviewRequestOptions{
		Reviewers: []string{"testOther"},
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)

	// Test Remove Review Request
	session2 := loginUser(t, "user4")
	token2 := getTokenForLoggedInUser(t, session2, auth_model.AccessTokenScopeWriteRepository)

	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", repo.OwnerName, repo.Name, pullIssue.Index), &api.PullReviewRequestOptions{
		Reviewers: []string{"user4"},
	}).AddTokenAuth(token2)
	MakeRequest(t, req, http.StatusNoContent)

	// doer is not admin
	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", repo.OwnerName, repo.Name, pullIssue.Index), &api.PullReviewRequestOptions{
		Reviewers: []string{"user8"},
	}).AddTokenAuth(token2)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", repo.OwnerName, repo.Name, pullIssue.Index), &api.PullReviewRequestOptions{
		Reviewers: []string{"user8"},
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// a collaborator can add/remove a review request
	pullIssue21 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 21})
	assert.NoError(t, pullIssue21.LoadAttributes(db.DefaultContext))
	pull21Repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pullIssue21.RepoID}) // repo60
	user38Session := loginUser(t, "user38")
	user38Token := getTokenForLoggedInUser(t, user38Session, auth_model.AccessTokenScopeWriteRepository)
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", pull21Repo.OwnerName, pull21Repo.Name, pullIssue21.Index), &api.PullReviewRequestOptions{
		Reviewers: []string{"user4@example.com"},
	}).AddTokenAuth(user38Token)
	MakeRequest(t, req, http.StatusCreated)

	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", pull21Repo.OwnerName, pull21Repo.Name, pullIssue21.Index), &api.PullReviewRequestOptions{
		Reviewers: []string{"user4@example.com"},
	}).AddTokenAuth(user38Token)
	MakeRequest(t, req, http.StatusNoContent)

	// the poster of the PR can add/remove a review request
	user39Session := loginUser(t, "user39")
	user39Token := getTokenForLoggedInUser(t, user39Session, auth_model.AccessTokenScopeWriteRepository)
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", pull21Repo.OwnerName, pull21Repo.Name, pullIssue21.Index), &api.PullReviewRequestOptions{
		Reviewers: []string{"user8"},
	}).AddTokenAuth(user39Token)
	MakeRequest(t, req, http.StatusCreated)

	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", pull21Repo.OwnerName, pull21Repo.Name, pullIssue21.Index), &api.PullReviewRequestOptions{
		Reviewers: []string{"user8"},
	}).AddTokenAuth(user39Token)
	MakeRequest(t, req, http.StatusNoContent)

	// user with read permission on pull requests unit can add/remove a review request
	pullIssue22 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 22})
	assert.NoError(t, pullIssue22.LoadAttributes(db.DefaultContext))
	pull22Repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pullIssue22.RepoID}) // repo61
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", pull22Repo.OwnerName, pull22Repo.Name, pullIssue22.Index), &api.PullReviewRequestOptions{
		Reviewers: []string{"user38"},
	}).AddTokenAuth(user39Token) // user39 is from a team with read permission on pull requests unit
	MakeRequest(t, req, http.StatusCreated)

	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", pull22Repo.OwnerName, pull22Repo.Name, pullIssue22.Index), &api.PullReviewRequestOptions{
		Reviewers: []string{"user38"},
	}).AddTokenAuth(user39Token) // user39 is from a team with read permission on pull requests unit
	MakeRequest(t, req, http.StatusNoContent)

	// Test team review request
	pullIssue12 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 12})
	assert.NoError(t, pullIssue12.LoadAttributes(db.DefaultContext))
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pullIssue12.RepoID})

	// Test add Team Review Request
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", repo3.OwnerName, repo3.Name, pullIssue12.Index), &api.PullReviewRequestOptions{
		TeamReviewers: []string{"team1", "owners"},
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)

	// Test add Team Review Request to not allowned
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", repo3.OwnerName, repo3.Name, pullIssue12.Index), &api.PullReviewRequestOptions{
		TeamReviewers: []string{"test_team"},
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	// Test add Team Review Request to not exist
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", repo3.OwnerName, repo3.Name, pullIssue12.Index), &api.PullReviewRequestOptions{
		TeamReviewers: []string{"not_exist_team"},
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)

	// Test Remove team Review Request
	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", repo3.OwnerName, repo3.Name, pullIssue12.Index), &api.PullReviewRequestOptions{
		TeamReviewers: []string{"team1"},
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// empty request test
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", repo3.OwnerName, repo3.Name, pullIssue12.Index), &api.PullReviewRequestOptions{}).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)

	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", repo3.OwnerName, repo3.Name, pullIssue12.Index), &api.PullReviewRequestOptions{}).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
}

func TestAPIPullReviewStayDismissed(t *testing.T) {
	// This test against issue https://github.com/go-gitea/gitea/issues/28542
	// where old reviews surface after a review request got dismissed.
	defer tests.PrepareTestEnv(t)()
	pullIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})
	assert.NoError(t, pullIssue.LoadAttributes(db.DefaultContext))
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pullIssue.RepoID})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session2 := loginUser(t, user2.LoginName)
	token2 := getTokenForLoggedInUser(t, session2, auth_model.AccessTokenScopeWriteRepository)
	user8 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})
	session8 := loginUser(t, user8.LoginName)
	token8 := getTokenForLoggedInUser(t, session8, auth_model.AccessTokenScopeWriteRepository)

	// user2 request user8
	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", repo.OwnerName, repo.Name, pullIssue.Index), &api.PullReviewRequestOptions{
		Reviewers: []string{user8.LoginName},
	}).AddTokenAuth(token2)
	MakeRequest(t, req, http.StatusCreated)

	reviewsCountCheck(t,
		"check we have only one review request",
		pullIssue.ID, user8.ID, 0, 1, 1, false)

	// user2 request user8 again, it is expected to be ignored
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", repo.OwnerName, repo.Name, pullIssue.Index), &api.PullReviewRequestOptions{
		Reviewers: []string{user8.LoginName},
	}).AddTokenAuth(token2)
	MakeRequest(t, req, http.StatusCreated)

	reviewsCountCheck(t,
		"check we have only one review request, even after re-request it again",
		pullIssue.ID, user8.ID, 0, 1, 1, false)

	// user8 reviews it as accept
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews", repo.OwnerName, repo.Name, pullIssue.Index), &api.CreatePullReviewOptions{
		Event: "APPROVED",
		Body:  "lgtm",
	}).AddTokenAuth(token8)
	MakeRequest(t, req, http.StatusOK)

	reviewsCountCheck(t,
		"check we have one valid approval",
		pullIssue.ID, user8.ID, 0, 0, 1, true)

	// emulate of auto-dismiss lgtm on a protected branch that where a pull just got an update
	_, err := db.GetEngine(db.DefaultContext).Where("issue_id = ? AND reviewer_id = ?", pullIssue.ID, user8.ID).
		Cols("dismissed").Update(&issues_model.Review{Dismissed: true})
	assert.NoError(t, err)

	// user2 request user8 again
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers", repo.OwnerName, repo.Name, pullIssue.Index), &api.PullReviewRequestOptions{
		Reviewers: []string{user8.LoginName},
	}).AddTokenAuth(token2)
	MakeRequest(t, req, http.StatusCreated)

	reviewsCountCheck(t,
		"check we have no valid approval and one review request",
		pullIssue.ID, user8.ID, 1, 1, 2, false)

	// user8 dismiss review
	permUser8, err := access_model.GetUserRepoPermission(db.DefaultContext, pullIssue.Repo, user8)
	assert.NoError(t, err)
	_, err = issue_service.ReviewRequest(db.DefaultContext, pullIssue, user8, &permUser8, user8, false)
	assert.NoError(t, err)

	reviewsCountCheck(t,
		"check new review request is now dismissed",
		pullIssue.ID, user8.ID, 1, 0, 1, false)

	// add a new valid approval
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews", repo.OwnerName, repo.Name, pullIssue.Index), &api.CreatePullReviewOptions{
		Event: "APPROVED",
		Body:  "lgtm",
	}).AddTokenAuth(token8)
	MakeRequest(t, req, http.StatusOK)

	reviewsCountCheck(t,
		"check that old reviews requests are deleted",
		pullIssue.ID, user8.ID, 1, 0, 2, true)

	// now add a change request witch should dismiss the approval
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews", repo.OwnerName, repo.Name, pullIssue.Index), &api.CreatePullReviewOptions{
		Event: "REQUEST_CHANGES",
		Body:  "please change XYZ",
	}).AddTokenAuth(token8)
	MakeRequest(t, req, http.StatusOK)

	reviewsCountCheck(t,
		"check that old reviews are dismissed",
		pullIssue.ID, user8.ID, 2, 0, 3, false)
}

func reviewsCountCheck(t *testing.T, name string, issueID, reviewerID int64, expectedDismissed, expectedRequested, expectedTotal int, expectApproval bool) {
	t.Run(name, func(t *testing.T) {
		unittest.AssertCountByCond(t, "review", builder.Eq{
			"issue_id":    issueID,
			"reviewer_id": reviewerID,
			"dismissed":   true,
		}, expectedDismissed)

		unittest.AssertCountByCond(t, "review", builder.Eq{
			"issue_id":    issueID,
			"reviewer_id": reviewerID,
		}, expectedTotal)

		unittest.AssertCountByCond(t, "review", builder.Eq{
			"issue_id":    issueID,
			"reviewer_id": reviewerID,
			"type":        issues_model.ReviewTypeRequest,
		}, expectedRequested)

		approvalCount := 0
		if expectApproval {
			approvalCount = 1
		}
		unittest.AssertCountByCond(t, "review", builder.Eq{
			"issue_id":    issueID,
			"reviewer_id": reviewerID,
			"type":        issues_model.ReviewTypeApprove,
			"dismissed":   false,
		}, approvalCount)
	})
}
