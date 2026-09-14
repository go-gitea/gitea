// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	issue_service "gitea.dev/services/issue"
	pull_service "gitea.dev/services/pull"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

func TestAPIPullReview(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	t.Run("General", testAPIPullReviewGeneral)
	t.Run("CommentReply", testAPIPullReviewCommentReply)
}

func testAPIPullReviewGeneral(t *testing.T) {
	pullIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})
	assert.NoError(t, pullIssue.LoadAttributes(t.Context()))
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pullIssue.RepoID})

	// test ListPullReviews
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews", repo.OwnerName, repo.Name, pullIssue.Index).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	reviews := DecodeJSON(t, resp, []*api.PullReview{})
	require.Len(t, reviews, 8)

	for _, r := range reviews {
		assert.Equal(t, pullIssue.HTMLURL(t.Context()), r.HTMLPullURL)
	}
	assert.EqualValues(t, 8, reviews[3].ID)
	assert.EqualValues(t, "APPROVED", reviews[3].State)
	assert.Equal(t, 0, reviews[3].CodeCommentsCount)
	assert.True(t, reviews[3].Stale)
	assert.False(t, reviews[3].Official)

	assert.EqualValues(t, 10, reviews[5].ID)
	assert.EqualValues(t, "REQUEST_CHANGES", reviews[5].State)
	assert.Equal(t, 1, reviews[5].CodeCommentsCount)
	assert.EqualValues(t, -1, reviews[5].Reviewer.ID) // ghost user
	assert.False(t, reviews[5].Stale)
	assert.True(t, reviews[5].Official)

	// test GetPullReview
	req = NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews/%d", repo.OwnerName, repo.Name, pullIssue.Index, reviews[3].ID).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	review := DecodeJSON(t, resp, &api.PullReview{})
	assert.Equal(t, reviews[3], review)

	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls/%d/reviews/%d", repo.OwnerName, repo.Name, pullIssue.Index, reviews[5].ID).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	review = DecodeJSON(t, resp, &api.PullReview{})
	assert.Equal(t, reviews[5], review)

	// test GetPullReviewComments
	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 7})
	req = NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews/%d/comments", repo.OwnerName, repo.Name, pullIssue.Index, 10).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	reviewComments := DecodeJSON(t, resp, []*api.PullReviewComment{})
	assert.Len(t, reviewComments, 1)
	assert.Equal(t, "Ghost", reviewComments[0].Poster.UserName)
	assert.Equal(t, "a review from a deleted user", reviewComments[0].Body)
	assert.Equal(t, comment.ID, reviewComments[0].ID)
	assert.EqualValues(t, comment.UpdatedUnix, reviewComments[0].Updated.Unix())
	assert.Equal(t, comment.HTMLURL(t.Context()), reviewComments[0].HTMLURL)

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
	review = DecodeJSON(t, resp, &api.PullReview{})
	assert.EqualValues(t, 6, review.ID)
	assert.EqualValues(t, "PENDING", review.State)
	assert.Equal(t, 3, review.CodeCommentsCount)

	// test SubmitPullReview
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews/%d", repo.OwnerName, repo.Name, pullIssue.Index, review.ID), &api.SubmitPullReviewOptions{
		Event: "APPROVED",
		Body:  "just two nits",
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	review = DecodeJSON(t, resp, &api.PullReview{})
	assert.EqualValues(t, 6, review.ID)
	assert.EqualValues(t, "APPROVED", review.State)
	assert.Equal(t, 3, review.CodeCommentsCount)

	// test dismiss review
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews/%d/dismissals", repo.OwnerName, repo.Name, pullIssue.Index, review.ID), &api.DismissPullReviewOptions{
		Message: "test",
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	review = DecodeJSON(t, resp, &api.PullReview{})
	assert.EqualValues(t, 6, review.ID)
	assert.True(t, review.Dismissed)

	// test dismiss review
	req = NewRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews/%d/undismissals", repo.OwnerName, repo.Name, pullIssue.Index, review.ID)).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	review = DecodeJSON(t, resp, &api.PullReview{})
	assert.EqualValues(t, 6, review.ID)
	assert.False(t, review.Dismissed)

	// test DeletePullReview
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews", repo.OwnerName, repo.Name, pullIssue.Index), &api.CreatePullReviewOptions{
		Body:  "just a comment",
		Event: "COMMENT",
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	review = DecodeJSON(t, resp, &api.PullReview{})
	assert.EqualValues(t, "COMMENT", review.State)
	assert.Equal(t, 0, review.CodeCommentsCount)
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

	resp = MakeRequest(t, req, http.StatusOK)
	commentReview := DecodeJSON(t, resp, &api.PullReview{})
	assert.EqualValues(t, "COMMENT", commentReview.State)
	assert.Equal(t, 2, commentReview.CodeCommentsCount)
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
	commentReview = DecodeJSON(t, resp, &api.PullReview{})
	assert.EqualValues(t, "COMMENT", commentReview.State)
	assert.Equal(t, 0, commentReview.CodeCommentsCount)
	assert.Equal(t, commentBody, commentReview.Body)
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
	assert.Equal(t, "review event COMMENT requires a body or a comment", errMap["message"])

	// test get review requests
	// to make it simple, use same api with get review
	pullIssue12 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 12})
	assert.NoError(t, pullIssue12.LoadAttributes(t.Context()))
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pullIssue12.RepoID})

	req = NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews", repo3.OwnerName, repo3.Name, pullIssue12.Index).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	reviews = DecodeJSON(t, resp, []*api.PullReview{})
	assert.EqualValues(t, 11, reviews[0].ID)
	assert.EqualValues(t, "REQUEST_REVIEW", reviews[0].State)
	assert.Equal(t, 0, reviews[0].CodeCommentsCount)
	assert.False(t, reviews[0].Stale)
	assert.True(t, reviews[0].Official)
	assert.Equal(t, "test_team", reviews[0].ReviewerTeam.Name)

	assert.EqualValues(t, 12, reviews[1].ID)
	assert.EqualValues(t, "REQUEST_REVIEW", reviews[1].State)
	assert.Equal(t, 0, reviews[0].CodeCommentsCount)
	assert.False(t, reviews[1].Stale)
	assert.True(t, reviews[1].Official)
	assert.EqualValues(t, 1, reviews[1].Reviewer.ID)
}

func TestAPIPullReviewRequest(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	pullIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})
	assert.NoError(t, pullIssue.LoadAttributes(t.Context()))
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
	assert.NoError(t, pullIssue21.LoadAttributes(t.Context()))
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
	assert.NoError(t, pullIssue22.LoadAttributes(t.Context()))
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
	assert.NoError(t, pullIssue12.LoadAttributes(t.Context()))
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

func TestAPIPullReviewCommentResolveEndpoints(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	ctx := t.Context()
	pullIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})
	require.NoError(t, pullIssue.LoadAttributes(ctx))
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pullIssue.RepoID})

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: pullIssue.PosterID})
	require.NoError(t, pullIssue.LoadPullRequest(ctx))
	gitRepo, err := git.OpenRepository(ctx, repo)
	require.NoError(t, err)
	defer gitRepo.Close()

	latestCommitID, err := gitRepo.GetRefCommitID(t.Context(), pullIssue.PullRequest.GetGitHeadRefName())
	require.NoError(t, err)

	codeComment, err := pull_service.CreateCodeComment(ctx, doer, gitRepo, pullIssue, 1, "resolve comment", "README.md", false, 0, latestCommitID, nil)
	require.NoError(t, err)
	require.NotNil(t, codeComment)

	session := loginUser(t, doer.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	resolveURL := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/comments/%d/resolve", repo.OwnerName, repo.Name, codeComment.ID)
	unresolveURL := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/comments/%d/unresolve", repo.OwnerName, repo.Name, codeComment.ID)

	req := NewRequest(t, http.MethodPost, resolveURL).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Verify comment is resolved
	updatedComment, err := issues_model.GetCommentByID(ctx, codeComment.ID)
	require.NoError(t, err)
	assert.NotZero(t, updatedComment.ResolveDoerID)
	assert.Equal(t, doer.ID, updatedComment.ResolveDoerID)

	// Resolving again should be idempotent
	MakeRequest(t, req, http.StatusNoContent)

	req = NewRequest(t, http.MethodPost, unresolveURL).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Verify comment is unresolved
	updatedComment, err = issues_model.GetCommentByID(ctx, codeComment.ID)
	require.NoError(t, err)
	assert.Zero(t, updatedComment.ResolveDoerID)

	// Unresolving again should be idempotent
	MakeRequest(t, req, http.StatusNoContent)

	// Non-existing comment ID
	req = NewRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/comments/999999/resolve", repo.OwnerName, repo.Name)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)

	// Non-code-comment
	plainComment, err := issue_service.CreateIssueComment(ctx, doer, repo, pullIssue, "not a review comment", nil)
	require.NoError(t, err)
	req = NewRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/comments/%d/resolve", repo.OwnerName, repo.Name, plainComment.ID)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusBadRequest)

	// Test permission check: use a user without write access for target repo to test 403 response
	unauthorizedUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	require.NotEqual(t, pullIssue.PosterID, unauthorizedUser.ID)

	unauthorizedSession := loginUser(t, unauthorizedUser.Name)
	unauthorizedToken := getTokenForLoggedInUser(t, unauthorizedSession, auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteRepository)

	req = NewRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/repos/%s/%s/issues/comments/%d", repo.OwnerName, repo.Name, plainComment.ID)).AddTokenAuth(unauthorizedToken)
	MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, http.MethodPost, resolveURL).AddTokenAuth(unauthorizedToken)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIPullReviewStayDismissed(t *testing.T) {
	// This test against issue https://github.com/go-gitea/gitea/issues/28542
	// where old reviews surface after a review request got dismissed.
	defer tests.PrepareTestEnv(t)()
	pullIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})
	assert.NoError(t, pullIssue.LoadAttributes(t.Context()))
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
	_, err := db.GetEngine(t.Context()).Where("issue_id = ? AND reviewer_id = ?", pullIssue.ID, user8.ID).
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
	permUser8, err := access_model.GetIndividualUserRepoPermission(t.Context(), pullIssue.Repo, user8)
	assert.NoError(t, err)
	_, err = issue_service.ReviewRequest(t.Context(), pullIssue, user8, &permUser8, user8, false)
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

func testAPIPullReviewCommentReply(t *testing.T) {
	pullIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})
	require.NoError(t, pullIssue.LoadRepo(t.Context()))
	require.NoError(t, pullIssue.LoadPullRequest(t.Context()))
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	gitRepo, err := git.OpenRepository(t.Context(), pullIssue.Repo)
	require.NoError(t, err)
	defer gitRepo.Close()

	commitID, err := gitRepo.GetRefCommitID(t.Context(), pullIssue.PullRequest.GetGitHeadRefName())
	require.NoError(t, err)

	parent, err := pull_service.CreateCodeComment(t.Context(), doer, gitRepo, pullIssue, 1, "parent comment", "README.md", false, 0, commitID, nil)
	require.NoError(t, err)
	require.NotZero(t, parent.ReviewID)

	repo := pullIssue.Repo
	session := loginUser(t, doer.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	url := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/comments/%d/replies", repo.OwnerName, repo.Name, pullIssue.Index, parent.ID)

	// happy path
	req := NewRequestWithJSON(t, http.MethodPost, url, &api.CreatePullReviewCommentReplyOptions{Body: "the reply"}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	reply := DecodeJSON(t, resp, &api.PullReviewComment{})
	assert.Equal(t, "the reply", reply.Body)
	assert.Equal(t, parent.ReviewID, reply.ReviewID)
	assert.Equal(t, "README.md", reply.Path)

	// empty body — caught by binding
	req = NewRequestWithJSON(t, http.MethodPost, url, &api.CreatePullReviewCommentReplyOptions{}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	// reply to a non-existent comment
	bad := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/comments/%d/replies", repo.OwnerName, repo.Name, pullIssue.Index, 999999)
	req = NewRequestWithJSON(t, http.MethodPost, bad, &api.CreatePullReviewCommentReplyOptions{Body: "x"}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)

	// reply to a code comment that belongs to a different PR — 404
	otherCodeComment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 4, Type: issues_model.CommentTypeCode})
	require.NotEqual(t, pullIssue.ID, otherCodeComment.IssueID)
	wrongPR := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/comments/%d/replies", repo.OwnerName, repo.Name, pullIssue.Index, otherCodeComment.ID)
	req = NewRequestWithJSON(t, http.MethodPost, wrongPR, &api.CreatePullReviewCommentReplyOptions{Body: "x"}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIPullReviewMutations(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	ctx := t.Context()
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})
	require.NoError(t, issue.LoadAttributes(ctx))
	require.NoError(t, issue.LoadPullRequest(ctx))
	repo := issue.Repo
	author := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	other := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	for _, user := range []*user_model.User{author, other} {
		permission, err := access_model.GetIndividualUserRepoPermission(ctx, repo, user)
		require.NoError(t, err)
		require.True(t, permission.CanRead(unit.TypePullRequests))
		require.False(t, permission.CanWrite(unit.TypePullRequests))
	}
	session := loginUser(t, author.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)
	otherToken := getUserToken(t, other.Name, auth_model.AccessTokenScopeWriteRepository)
	ownerToken := getUserToken(t, repo.OwnerName, auth_model.AccessTokenScopeWriteRepository)
	adminToken := getUserToken(t, "user1", auth_model.AccessTokenScopeWriteRepository)
	gitRepo, err := git.OpenRepository(ctx, repo)
	require.NoError(t, err)
	defer gitRepo.Close()
	commitID, err := gitRepo.GetRefCommitID(ctx, issue.PullRequest.GetGitHeadRefName())
	require.NoError(t, err)

	// UI drafts do not have a summary comment until submission.
	review, err := issues_model.CreateReview(ctx, issues_model.CreateReviewOptions{
		Type: issues_model.ReviewTypePending, Issue: issue, Reviewer: author, CommitID: commitID,
	})
	require.NoError(t, err)
	pullsURL := fmt.Sprintf("/api/v1/repos/%s/%s/pulls", repo.OwnerName, repo.Name)
	reviewsURL := fmt.Sprintf("%s/%d/reviews", pullsURL, issue.Index)
	reviewURL := fmt.Sprintf("%s/%d", reviewsURL, review.ID)
	request := func(t *testing.T, method, url, auth string, body any, status int) *httptest.ResponseRecorder {
		t.Helper()
		return MakeRequest(t, NewRequestWithJSON(t, method, url, body).AddTokenAuth(auth), status)
	}
	reviewEdit := api.EditPullReviewOptions{Body: new("edited summary")}
	commentEdit := api.EditPullReviewCommentOptions{Body: new("edited comment")}
	appendOpts := api.CreatePullReviewComment{Path: "README.md", Body: "new comment", NewLineNum: 1}
	draft := DecodeJSON(t, request(t, "PATCH", reviewURL, token, reviewEdit, http.StatusOK), &api.PullReview{})
	assert.Equal(t, "edited summary", draft.Body)
	assert.Equal(t, api.ReviewStatePending, draft.State)
	assert.Equal(t, commitID, draft.CommitID)
	unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: review.ID, Content: draft.Body})
	var comments []*api.PullReviewComment
	for _, opts := range []api.CreatePullReviewComment{appendOpts, {Path: "README.md", Body: "old comment", OldLineNum: 1}} {
		comment := DecodeJSON(t, request(t, "POST", reviewURL+"/comments", token, opts, http.StatusCreated), &api.PullReviewComment{})
		require.NotNil(t, comment.Poster)
		assert.Equal(t, author.ID, comment.Poster.ID)
		assert.Equal(t, review.ID, comment.ReviewID)
		assert.Equal(t, opts.Body, comment.Body)
		assert.Equal(t, opts.Path, comment.Path)
		assert.EqualValues(t, opts.NewLineNum, comment.LineNum)
		assert.EqualValues(t, opts.OldLineNum, comment.OldLineNum)
		assert.NotEmpty(t, comment.DiffHunk)
		assert.NotEmpty(t, comment.HTMLURL)
		comments = append(comments, comment)
	}
	unittest.AssertCount(t, &issues_model.Comment{ReviewID: review.ID, Type: issues_model.CommentTypeReview}, 0)
	assert.ElementsMatch(t, comments, DecodeJSON(t, request(t, "GET", reviewURL+"/comments", token, nil, http.StatusOK), []*api.PullReviewComment{}))
	commentURL := fmt.Sprintf("%s/comments/%d", pullsURL, comments[0].ID)
	mutations := []struct {
		name, method, url string
		body              any
	}{
		{"Review", "PATCH", reviewURL, reviewEdit},
		{"Comment", "PATCH", commentURL, commentEdit},
		{"Delete", "DELETE", commentURL, nil},
		{"Append", "POST", reviewURL + "/comments", appendOpts},
	}
	t.Run("PendingPrivacy", func(t *testing.T) {
		for _, auth := range []string{ownerToken, otherToken} {
			for _, suffix := range []string{"", "/comments"} {
				request(t, "GET", reviewURL+suffix, auth, nil, http.StatusNotFound)
			}
			for _, mutation := range mutations {
				request(t, mutation.method, mutation.url, auth, mutation.body, http.StatusNotFound)
			}
		}
		request(t, "GET", reviewURL, "", nil, http.StatusNotFound)
		request(t, "GET", reviewURL, adminToken, nil, http.StatusOK)
		request(t, "PATCH", commentURL, adminToken, struct{}{}, http.StatusOK)
		request(t, "POST", reviewURL+"/comments", adminToken, appendOpts, http.StatusForbidden)
		request(t, "POST", commentURL+"/resolve", ownerToken, nil, http.StatusNotFound)
		request(t, "POST", fmt.Sprintf("%s/%d/comments/%d/replies", pullsURL, issue.Index, comments[0].ID), ownerToken,
			api.CreatePullReviewCommentReplyOptions{Body: "private reply"}, http.StatusNotFound)
	})

	commentCount := unittest.GetCount(t, &issues_model.Comment{})
	reviewCount := unittest.GetCount(t, &issues_model.Review{})
	for _, tc := range []struct {
		name, body, path string
		old, new         int64
	}{
		{"EmptyBody", "", "README.md", 0, 1},
		{"WhitespaceBody", " \t\r\n", "README.md", 0, 1},
		{"EmptyPath", "body", "", 0, 1},
		{"Dot", "body", ".", 0, 1},
		{"Traversal", "body", "../README.md", 0, 1},
		{"Absolute", "body", "/README.md", 0, 1},
		{"NUL", "body", "README.md\x00", 0, 1},
		{"MissingFile", "body", "missing.txt", 0, 1},
		{"NoPosition", "body", "README.md", 0, 0},
		{"BothSides", "body", "README.md", 1, 1},
		{"NegativePosition", "body", "README.md", 0, -1},
		{"NegativeOtherSide", "body", "README.md", -1, 1},
		{"NewOutOfRange", "body", "README.md", 0, 999999},
		{"OldOutOfRange", "body", "README.md", 999999, 0},
	} {
		t.Run("InvalidComment/"+tc.name, func(t *testing.T) {
			request(t, "POST", reviewURL+"/comments", token, api.CreatePullReviewComment{
				Body: tc.body, Path: tc.path, OldLineNum: tc.old, NewLineNum: tc.new,
			}, http.StatusUnprocessableEntity)
			unittest.AssertCount(t, &issues_model.Comment{}, commentCount)
			unittest.AssertCount(t, &issues_model.Review{}, reviewCount)
		})
	}
	request(t, "PATCH", reviewsURL+"/22", token, reviewEdit, http.StatusUnprocessableEntity)
	request(t, "POST", reviewsURL+"/22/comments", token, appendOpts, http.StatusUnprocessableEntity)
	for _, url := range []string{
		fmt.Sprintf("%s/2/reviews/%d", pullsURL, review.ID),
		fmt.Sprintf("/api/v1/repos/org3/repo3/pulls/2/reviews/%d", review.ID),
		reviewsURL + "/999999",
	} {
		request(t, "PATCH", url, adminToken, reviewEdit, http.StatusNotFound)
		request(t, "POST", url+"/comments", adminToken, appendOpts, http.StatusNotFound)
	}
	for _, target := range []struct {
		url    string
		status int
	}{
		{fmt.Sprintf("/api/v1/repos/org3/repo3/pulls/comments/%d", comments[0].ID), http.StatusNotFound},
		{pullsURL + "/comments/999999", http.StatusNotFound},
		{pullsURL + "/comments/2", http.StatusBadRequest},
		{pullsURL + "/comments/10", http.StatusBadRequest},
	} {
		request(t, "PATCH", target.url, adminToken, commentEdit, target.status)
		request(t, "DELETE", target.url, adminToken, nil, target.status)
	}
	issue.IsLocked = true
	require.NoError(t, issues_model.UpdateIssueCols(ctx, issue, "is_locked"))
	request(t, "POST", reviewURL+"/comments", token, appendOpts, http.StatusForbidden)
	issue.IsLocked = false
	require.NoError(t, issues_model.UpdateIssueCols(ctx, issue, "is_locked"))
	blocking := &user_model.Blocking{BlockerID: repo.OwnerID, BlockeeID: author.ID}
	require.NoError(t, db.Insert(ctx, blocking))
	request(t, "POST", reviewURL+"/comments", token, appendOpts, http.StatusForbidden)
	request(t, "PATCH", reviewURL, token, reviewEdit, http.StatusForbidden)
	request(t, "PATCH", commentURL, token, commentEdit, http.StatusForbidden)
	_, err = db.DeleteByBean(ctx, blocking)
	require.NoError(t, err)
	unittest.AssertCount(t, &issues_model.Comment{}, commentCount)
	unittest.AssertCount(t, &issues_model.Review{}, reviewCount)
	draft = DecodeJSON(t, request(t, "GET", reviewURL, token, nil, http.StatusOK), &api.PullReview{})
	assert.Equal(t, api.ReviewStatePending, draft.State)
	assert.Equal(t, "edited summary", draft.Body)
	assert.Equal(t, 2, draft.CodeCommentsCount)

	// The existing create API can leave an earlier summary linked to the same review.
	request(t, "POST", reviewsURL, token, api.CreatePullReviewOptions{Body: "pending header"}, http.StatusOK)
	request(t, "POST", reviewURL, token, api.SubmitPullReviewOptions{Event: api.ReviewStateApproved, Body: "published summary"}, http.StatusOK)
	for _, mutation := range mutations {
		t.Run("Authorization/"+mutation.name, func(t *testing.T) {
			request(t, mutation.method, mutation.url, "", mutation.body, http.StatusUnauthorized)
			for _, auth := range []string{readToken, otherToken} {
				request(t, mutation.method, mutation.url, auth, mutation.body, http.StatusForbidden)
			}
		})
	}
	request(t, "POST", reviewURL+"/comments", ownerToken, appendOpts, http.StatusForbidden)
	request(t, "POST", reviewURL+"/comments", adminToken, appendOpts, http.StatusForbidden)
	request(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/collaborators/%s", repo.OwnerName, repo.Name, other.Name), ownerToken,
		api.AddCollaboratorOption{Permission: new(api.RepoWritePermissionWrite)}, http.StatusNoContent)
	permission, err := access_model.GetIndividualUserRepoPermission(ctx, repo, other)
	require.NoError(t, err)
	require.True(t, permission.CanWrite(unit.TypePullRequests))
	require.False(t, permission.IsAdmin())
	request(t, "POST", commentURL+"/resolve", otherToken, nil, http.StatusNoContent)
	_, err = db.GetEngine(ctx).ID(review.ID).Cols("official", "dismissed", "stale").Update(&issues_model.Review{Official: true, Dismissed: true, Stale: true})
	require.NoError(t, err)
	before := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: review.ID})
	headers, err := issues_model.FindComments(ctx, &issues_model.FindCommentsOptions{ReviewID: review.ID, Type: issues_model.CommentTypeReview})
	require.NoError(t, err)
	require.Len(t, headers, 2)
	for _, step := range []struct {
		auth, want string
		body       *string
		history    int
	}{
		{token, "edited", new("edited"), 2},
		{token, "edited", nil, 2},
		{token, "", new(""), 3},
		{otherToken, "moderated", new("moderated"), 4},
	} {
		var reviewBody, commentBody any = api.EditPullReviewOptions{Body: step.body}, api.EditPullReviewCommentOptions{Body: step.body}
		if step.body == nil {
			reviewBody, commentBody = struct{}{}, struct{}{}
		}
		edited := DecodeJSON(t, request(t, "PATCH", reviewURL, step.auth, reviewBody, http.StatusOK), &api.PullReview{})
		assert.Equal(t, step.want, edited.Body)
		assert.Equal(t, edited, DecodeJSON(t, request(t, "GET", reviewURL, step.auth, nil, http.StatusOK), &api.PullReview{}))
		stored := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: review.ID})
		assert.Equal(t, step.want, stored.Content)
		stored.Content, stored.UpdatedUnix = before.Content, before.UpdatedUnix
		assert.Equal(t, before, stored)
		page := session.MakeRequest(t, NewRequest(t, "GET", issue.HTMLURL(ctx)), http.StatusOK)
		doc := NewHTMLParser(t, page.Body)
		for _, header := range headers {
			storedHeader := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: header.ID})
			assert.Equal(t, step.want, storedHeader.Content)
			assert.Equal(t, step.want, doc.Find("#"+header.HashTag()+"-raw").Text())
			unittest.AssertCount(t, &issues_model.ContentHistory{CommentID: header.ID}, step.history)
			unittest.AssertExistsAndLoadBean(t, &issues_model.ContentHistory{CommentID: header.ID, PosterID: author.ID, ContentText: header.Content, IsFirstCreated: true})
		}
		comment := DecodeJSON(t, request(t, "PATCH", commentURL, step.auth, commentBody, http.StatusOK), &api.PullReviewComment{})
		require.NotNil(t, comment.Poster)
		require.NotNil(t, comment.Resolver)
		assert.Equal(t, author.ID, comment.Poster.ID)
		assert.Equal(t, other.ID, comment.Resolver.ID)
		assert.Equal(t, step.want, comment.Body)
		comments[0] = comment
		roundtrip := DecodeJSON(t, request(t, "GET", reviewURL+"/comments", step.auth, nil, http.StatusOK), []*api.PullReviewComment{})
		require.Len(t, roundtrip, 2)
		assert.Contains(t, roundtrip, comment)
		unittest.AssertCount(t, &issues_model.ContentHistory{CommentID: comment.ID}, step.history)
	}
	pending, err := issues_model.CreateReview(ctx, issues_model.CreateReviewOptions{
		Type: issues_model.ReviewTypePending, Issue: issue, Reviewer: author, CommitID: commitID, Content: "another draft",
	})
	require.NoError(t, err)
	pending = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: pending.ID})
	appendOpts.Body = "late comment"
	late := DecodeJSON(t, request(t, "POST", reviewURL+"/comments", token, appendOpts, http.StatusCreated), &api.PullReviewComment{})
	assert.Equal(t, review.ID, late.ReviewID)
	assert.Equal(t, pending, unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: pending.ID}))
	unittest.AssertCount(t, &issues_model.Comment{ReviewID: pending.ID}, 0)
	for i, auth := range []string{token, otherToken} {
		resp := request(t, "DELETE", fmt.Sprintf("%s/comments/%d", pullsURL, comments[i].ID), auth, nil, http.StatusNoContent)
		assert.Empty(t, resp.Body.String())
		unittest.AssertNotExistsBean(t, &issues_model.Comment{ID: comments[i].ID})
	}
	assert.Equal(t, []*api.PullReviewComment{late}, DecodeJSON(t, request(t, "GET", reviewURL+"/comments", token, nil, http.StatusOK), []*api.PullReviewComment{}))
	request(t, "DELETE", fmt.Sprintf("%s/comments/%d", pullsURL, late.ID), token, nil, http.StatusNoContent)
	unittest.AssertCount(t, &issues_model.Comment{ReviewID: review.ID, Type: issues_model.CommentTypeCode}, 0)
	retained := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: review.ID, Content: "moderated"})
	retained.Content, retained.UpdatedUnix = before.Content, before.UpdatedUnix
	assert.Equal(t, before, retained)
	unittest.AssertCount(t, &issues_model.Comment{ReviewID: review.ID, Type: issues_model.CommentTypeReview}, 2)
}

func TestAPIPullReviewCommentSnapshot(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	ctx := t.Context()
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pr.BaseRepoID})
	gitRepo, err := git.OpenRepository(ctx, repo)
	require.NoError(t, err)
	defer gitRepo.Close()
	require.NoError(t, git.ForceFastImport(ctx, repo.CodeStorageRepo(), []git.FastImportCommit{
		{Ref: "refs/heads/comment-base", Files: []git.FastImportFile{
			{Path: "changed.txt", Content: "old one\nanchor\nold two\n"},
			{Path: "deleted.txt", Content: "deleted one\ndeleted two\n"},
		}},
		{Ref: "refs/heads/comment-snapshot", Files: []git.FastImportFile{{Path: "changed.txt", Content: "snapshot one\nanchor\nsnapshot two\n"}}},
		{Ref: "refs/heads/comment-advanced", Files: []git.FastImportFile{
			{Path: "changed.txt", Content: "advanced only\n"},
			{Path: "future.txt", Content: "not in the review\n"},
		}},
	}))
	base, err := gitRepo.GetRefCommitID(ctx, "refs/heads/comment-base")
	require.NoError(t, err)
	// Fast-import creates roots; connect the snapshots into a real PR history.
	child := func(ref, parent string) string {
		t.Helper()
		tree, err := gitRepo.GetTree(ctx, ref)
		require.NoError(t, err)
		identity := &git.Signature{Name: "Gitea", Email: "gitea@example.com"}
		id, err := gitRepo.CommitTree(ctx, identity, identity, tree, git.CommitTreeOpts{Parents: []string{parent}, Message: ref, NoGPGSign: true})
		require.NoError(t, err)
		return id.String()
	}
	snapshot := child("refs/heads/comment-snapshot", base)
	advanced := child("refs/heads/comment-advanced", snapshot)
	pr.BaseBranch, pr.MergeBase = "comment-base", base
	_, err = db.GetEngine(ctx).ID(pr.ID).Cols("base_branch", "merge_base").Update(pr)
	require.NoError(t, err)
	require.NoError(t, git.UpdateRef(ctx, repo, pr.GetGitHeadRefName(), snapshot))
	token := getUserToken(t, "user5", auth_model.AccessTokenScopeWriteRepository)
	reviewsURL := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews", repo.OwnerName, repo.Name, pr.Index)
	req := NewRequestWithJSON(t, "POST", reviewsURL, api.CreatePullReviewOptions{Event: api.ReviewStateComment, Body: "historical review", CommitID: snapshot}).AddTokenAuth(token)
	review := DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.PullReview{})
	require.NoError(t, git.UpdateRef(ctx, repo, pr.GetGitHeadRefName(), advanced))
	commentsURL := fmt.Sprintf("%s/%d/comments", reviewsURL, review.ID)
	for _, tc := range []struct {
		name, path, patch string
		old, new          int64
		invalidated       bool
	}{
		{"NewSide", "changed.txt", "+snapshot two", 0, 3, true},
		{"OldSide", "changed.txt", "+snapshot one", 3, 0, true},
		{"DeletedOldFile", "deleted.txt", "-deleted two", 2, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := NewRequestWithJSON(t, "POST", commentsURL, api.CreatePullReviewComment{Path: tc.path, Body: tc.name, OldLineNum: tc.old, NewLineNum: tc.new}).AddTokenAuth(token)
			comment := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.PullReviewComment{})
			assert.Equal(t, review.ID, comment.ReviewID)
			assert.Equal(t, snapshot, comment.CommitID)
			assert.EqualValues(t, tc.old, comment.OldLineNum)
			assert.EqualValues(t, tc.new, comment.LineNum)
			assert.Contains(t, comment.DiffHunk, tc.patch)
			assert.NotContains(t, comment.DiffHunk, "advanced only")
			stored := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: comment.ID})
			assert.Equal(t, tc.new-tc.old, stored.Line)
			assert.Contains(t, stored.Patch, tc.patch)
			assert.Equal(t, tc.invalidated, stored.Invalidated)
		})
	}
	for _, opts := range []api.CreatePullReviewComment{
		{Path: "future.txt", Body: "not in snapshot", NewLineNum: 1},
		{Path: "deleted.txt", Body: "only on old side", NewLineNum: 1},
		{Path: "changed.txt", Body: "outside snapshot", OldLineNum: 4},
	} {
		req := NewRequestWithJSON(t, "POST", commentsURL, opts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusUnprocessableEntity)
		unittest.AssertCount(t, &issues_model.Comment{ReviewID: review.ID, Type: issues_model.CommentTypeCode}, 3)
	}
	got := DecodeJSON(t, MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("%s/%d", reviewsURL, review.ID)).AddTokenAuth(token), http.StatusOK), &api.PullReview{})
	review.CodeCommentsCount = 3
	assert.Equal(t, review, got)

	require.NoError(t, pr.LoadIssue(ctx))
	pr.Issue.Repo = repo
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	pending, err := issues_model.CreateReview(ctx, issues_model.CreateReviewOptions{
		Type: issues_model.ReviewTypePending, Issue: pr.Issue, Reviewer: doer, CommitID: snapshot,
	})
	require.NoError(t, err)
	comment, err := pull_service.CreateCodeComment(ctx, doer, gitRepo, pr.Issue, 1, "comment on the current Files UI", "future.txt", true, 0, advanced, nil)
	require.NoError(t, err)
	assert.Equal(t, pending.ID, comment.ReviewID)
	assert.Contains(t, comment.Patch, "+not in the review")
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
