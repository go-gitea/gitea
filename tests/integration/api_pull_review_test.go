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
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIPullReview(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	pullIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})
	assert.NoError(t, pullIssue.LoadAttributes(db.DefaultContext))
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pullIssue.RepoID})

	// test ListPullReviews
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)
	req := NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token)
	resp := MakeRequest(t, req, http.StatusOK)

	var reviews []*api.PullReview
	DecodeJSON(t, resp, &reviews)
	if !assert.Len(t, reviews, 6) {
		return
	}
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
	req = NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews/%d?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, reviews[3].ID, token)
	resp = MakeRequest(t, req, http.StatusOK)
	var review api.PullReview
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, *reviews[3], review)

	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls/%d/reviews/%d?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, reviews[5].ID, token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, *reviews[5], review)

	// test GetPullReviewComments
	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 7})
	req = NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews/%d/comments?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, 10, token)
	resp = MakeRequest(t, req, http.StatusOK)
	var reviewComments []*api.PullReviewComment
	DecodeJSON(t, resp, &reviewComments)
	assert.Len(t, reviewComments, 1)
	assert.EqualValues(t, "Ghost", reviewComments[0].Poster.UserName)
	assert.EqualValues(t, "a review from a deleted user", reviewComments[0].Body)
	assert.EqualValues(t, comment.ID, reviewComments[0].ID)
	assert.EqualValues(t, comment.UpdatedUnix, reviewComments[0].Updated.Unix())
	assert.EqualValues(t, comment.HTMLURL(), reviewComments[0].HTMLURL)

	// test CreatePullReview
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.CreatePullReviewOptions{
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
	})
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, 6, review.ID)
	assert.EqualValues(t, "PENDING", review.State)
	assert.EqualValues(t, 3, review.CodeCommentsCount)

	// test SubmitPullReview
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews/%d?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, review.ID, token), &api.SubmitPullReviewOptions{
		Event: "APPROVED",
		Body:  "just two nits",
	})
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, 6, review.ID)
	assert.EqualValues(t, "APPROVED", review.State)
	assert.EqualValues(t, 3, review.CodeCommentsCount)

	// test dismiss review
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews/%d/dismissals?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, review.ID, token), &api.DismissPullReviewOptions{
		Message: "test",
	})
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, 6, review.ID)
	assert.True(t, review.Dismissed)

	// test dismiss review
	req = NewRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews/%d/undismissals?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, review.ID, token))
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, 6, review.ID)
	assert.False(t, review.Dismissed)

	// test DeletePullReview
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.CreatePullReviewOptions{
		Body:  "just a comment",
		Event: "COMMENT",
	})
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, "COMMENT", review.State)
	assert.EqualValues(t, 0, review.CodeCommentsCount)
	req = NewRequestf(t, http.MethodDelete, "/api/v1/repos/%s/%s/pulls/%d/reviews/%d?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, review.ID, token)
	MakeRequest(t, req, http.StatusNoContent)

	// test CreatePullReview Comment without body but with comments
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.CreatePullReviewOptions{
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
	})
	var commentReview api.PullReview

	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &commentReview)
	assert.EqualValues(t, "COMMENT", commentReview.State)
	assert.EqualValues(t, 2, commentReview.CodeCommentsCount)
	assert.Empty(t, commentReview.Body)
	assert.False(t, commentReview.Dismissed)

	// test CreatePullReview Comment with body but without comments
	commentBody := "This is a body of the comment."
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.CreatePullReviewOptions{
		Body:     commentBody,
		Event:    "COMMENT",
		Comments: []api.CreatePullReviewComment{},
	})

	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &commentReview)
	assert.EqualValues(t, "COMMENT", commentReview.State)
	assert.EqualValues(t, 0, commentReview.CodeCommentsCount)
	assert.EqualValues(t, commentBody, commentReview.Body)
	assert.False(t, commentReview.Dismissed)

	// test CreatePullReview Comment without body and no comments
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.CreatePullReviewOptions{
		Body:     "",
		Event:    "COMMENT",
		Comments: []api.CreatePullReviewComment{},
	})
	resp = MakeRequest(t, req, http.StatusUnprocessableEntity)
	errMap := make(map[string]interface{})
	json.Unmarshal(resp.Body.Bytes(), &errMap)
	assert.EqualValues(t, "review event COMMENT requires a body or a comment", errMap["message"].(string))

	// test get review requests
	// to make it simple, use same api with get review
	pullIssue12 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 12})
	assert.NoError(t, pullIssue12.LoadAttributes(db.DefaultContext))
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pullIssue12.RepoID})

	req = NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews?token=%s", repo3.OwnerName, repo3.Name, pullIssue12.Index, token)
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
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)
	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.PullReviewRequestOptions{
		Reviewers: []string{"user4@example.com", "user8"},
	})
	MakeRequest(t, req, http.StatusCreated)

	// poster of pr can't be reviewer
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.PullReviewRequestOptions{
		Reviewers: []string{"user1"},
	})
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	// test user not exist
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.PullReviewRequestOptions{
		Reviewers: []string{"testOther"},
	})
	MakeRequest(t, req, http.StatusNotFound)

	// Test Remove Review Request
	session2 := loginUser(t, "user4")
	token2 := getTokenForLoggedInUser(t, session2, auth_model.AccessTokenScopeRepo)

	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token2), &api.PullReviewRequestOptions{
		Reviewers: []string{"user4"},
	})
	MakeRequest(t, req, http.StatusNoContent)

	// doer is not admin
	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token2), &api.PullReviewRequestOptions{
		Reviewers: []string{"user8"},
	})
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.PullReviewRequestOptions{
		Reviewers: []string{"user8"},
	})
	MakeRequest(t, req, http.StatusNoContent)

	// Test team review request
	pullIssue12 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 12})
	assert.NoError(t, pullIssue12.LoadAttributes(db.DefaultContext))
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pullIssue12.RepoID})

	// Test add Team Review Request
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo3.OwnerName, repo3.Name, pullIssue12.Index, token), &api.PullReviewRequestOptions{
		TeamReviewers: []string{"team1", "owners"},
	})
	MakeRequest(t, req, http.StatusCreated)

	// Test add Team Review Request to not allowned
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo3.OwnerName, repo3.Name, pullIssue12.Index, token), &api.PullReviewRequestOptions{
		TeamReviewers: []string{"test_team"},
	})
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	// Test add Team Review Request to not exist
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo3.OwnerName, repo3.Name, pullIssue12.Index, token), &api.PullReviewRequestOptions{
		TeamReviewers: []string{"not_exist_team"},
	})
	MakeRequest(t, req, http.StatusNotFound)

	// Test Remove team Review Request
	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo3.OwnerName, repo3.Name, pullIssue12.Index, token), &api.PullReviewRequestOptions{
		TeamReviewers: []string{"team1"},
	})
	MakeRequest(t, req, http.StatusNoContent)

	// empty request test
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo3.OwnerName, repo3.Name, pullIssue12.Index, token), &api.PullReviewRequestOptions{})
	MakeRequest(t, req, http.StatusCreated)

	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo3.OwnerName, repo3.Name, pullIssue12.Index, token), &api.PullReviewRequestOptions{})
	MakeRequest(t, req, http.StatusNoContent)
}
