// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIListRepoComments(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{},
		unittest.Cond("type = ?", issues_model.CommentTypeComment))
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: comment.IssueID})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	link, _ := url.Parse(fmt.Sprintf("/api/v1/repos/%s/%s/issues/comments", repoOwner.Name, repo.Name))
	req := NewRequest(t, "GET", link.String())
	resp := MakeRequest(t, req, http.StatusOK)

	var apiComments []*api.Comment
	DecodeJSON(t, resp, &apiComments)
	assert.Len(t, apiComments, 2)
	for _, apiComment := range apiComments {
		c := &issues_model.Comment{ID: apiComment.ID}
		unittest.AssertExistsAndLoadBean(t, c,
			unittest.Cond("type = ?", issues_model.CommentTypeComment))
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: c.IssueID, RepoID: repo.ID})
	}

	// test before and since filters
	query := url.Values{}
	before := "2000-01-01T00:00:11+00:00" // unix: 946684811
	since := "2000-01-01T00:00:12+00:00"  // unix: 946684812
	query.Add("before", before)
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiComments)
	assert.Len(t, apiComments, 1)
	assert.EqualValues(t, 2, apiComments[0].ID)

	query.Del("before")
	query.Add("since", since)
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiComments)
	assert.Len(t, apiComments, 1)
	assert.EqualValues(t, 3, apiComments[0].ID)
}

func TestAPIListIssueComments(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{},
		unittest.Cond("type = ?", issues_model.CommentTypeComment))
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: comment.IssueID})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	token := getUserToken(t, repoOwner.Name, auth_model.AccessTokenScopeReadIssue)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/%d/comments", repoOwner.Name, repo.Name, issue.Index).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var comments []*api.Comment
	DecodeJSON(t, resp, &comments)
	expectedCount := unittest.GetCount(t, &issues_model.Comment{IssueID: issue.ID},
		unittest.Cond("type = ?", issues_model.CommentTypeComment))
	assert.Len(t, comments, expectedCount)
}

func TestAPICreateComment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	const commentBody = "Comment body"

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	token := getUserToken(t, repoOwner.Name, auth_model.AccessTokenScopeWriteIssue)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/comments",
		repoOwner.Name, repo.Name, issue.Index)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"body": commentBody,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)

	var updatedComment api.Comment
	DecodeJSON(t, resp, &updatedComment)
	assert.EqualValues(t, commentBody, updatedComment.Body)
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: updatedComment.ID, IssueID: issue.ID, Content: commentBody})

	t.Run("BlockedByRepoOwner", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		user34 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 34})
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})

		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/comments", repo.OwnerName, repo.Name, issue.Index), map[string]string{
			"body": commentBody,
		}).AddTokenAuth(getUserToken(t, user34.Name, auth_model.AccessTokenScopeWriteRepository))
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("BlockedByIssuePoster", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		user34 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 34})
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 13})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})

		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/comments", repo.OwnerName, repo.Name, issue.Index), map[string]string{
			"body": commentBody,
		}).AddTokenAuth(getUserToken(t, user34.Name, auth_model.AccessTokenScopeWriteRepository))
		MakeRequest(t, req, http.StatusForbidden)
	})
}

func TestAPIGetComment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 2})
	assert.NoError(t, comment.LoadIssue(db.DefaultContext))
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: comment.Issue.RepoID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	token := getUserToken(t, repoOwner.Name, auth_model.AccessTokenScopeReadIssue)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/comments/%d", repoOwner.Name, repo.Name, comment.ID)
	MakeRequest(t, req, http.StatusOK)
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/comments/%d", repoOwner.Name, repo.Name, comment.ID).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var apiComment api.Comment
	DecodeJSON(t, resp, &apiComment)

	assert.NoError(t, comment.LoadPoster(db.DefaultContext))
	expect := convert.ToAPIComment(db.DefaultContext, repo, comment, nil)

	assert.Equal(t, expect.ID, apiComment.ID)
	assert.Equal(t, expect.Poster.FullName, apiComment.Poster.FullName)
	assert.Equal(t, expect.Body, apiComment.Body)
	assert.Equal(t, expect.Created.Unix(), apiComment.Created.Unix())
}

func TestAPIGetSystemUserComment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	for _, systemUser := range []*user_model.User{
		user_model.NewGhostUser(),
		user_model.NewActionsUser(),
	} {
		body := fmt.Sprintf("Hello %s", systemUser.Name)
		comment, err := issues_model.CreateComment(db.DefaultContext, &issues_model.CreateCommentOptions{
			Type:    issues_model.CommentTypeComment,
			Doer:    systemUser,
			Repo:    repo,
			Issue:   issue,
			Content: body,
		})
		assert.NoError(t, err)

		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/comments/%d", repoOwner.Name, repo.Name, comment.ID)
		resp := MakeRequest(t, req, http.StatusOK)

		var apiComment api.Comment
		DecodeJSON(t, resp, &apiComment)

		if assert.NotNil(t, apiComment.Poster) {
			if assert.Equal(t, systemUser.ID, apiComment.Poster.ID) {
				assert.NoError(t, comment.LoadPoster(db.DefaultContext))
				assert.Equal(t, systemUser.Name, apiComment.Poster.UserName)
			}
		}
		assert.Equal(t, body, apiComment.Body)
	}
}

func TestAPIEditComment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	const newCommentBody = "This is the new comment body"

	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 8},
		unittest.Cond("type = ?", issues_model.CommentTypeComment))
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: comment.IssueID})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	t.Run("UnrelatedCommentID", func(t *testing.T) {
		// Using the ID of a comment that does not belong to the repository must fail
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
		repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		token := getUserToken(t, repoOwner.Name, auth_model.AccessTokenScopeWriteIssue)
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/comments/%d",
			repoOwner.Name, repo.Name, comment.ID)
		req := NewRequestWithValues(t, "PATCH", urlStr, map[string]string{
			"body": newCommentBody,
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	token := getUserToken(t, repoOwner.Name, auth_model.AccessTokenScopeWriteIssue)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/comments/%d",
		repoOwner.Name, repo.Name, comment.ID)
	req := NewRequestWithValues(t, "PATCH", urlStr, map[string]string{
		"body": newCommentBody,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var updatedComment api.Comment
	DecodeJSON(t, resp, &updatedComment)
	assert.EqualValues(t, comment.ID, updatedComment.ID)
	assert.EqualValues(t, newCommentBody, updatedComment.Body)
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: comment.ID, IssueID: issue.ID, Content: newCommentBody})
}

func TestAPIDeleteComment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 8},
		unittest.Cond("type = ?", issues_model.CommentTypeComment))
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: comment.IssueID})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	t.Run("UnrelatedCommentID", func(t *testing.T) {
		// Using the ID of a comment that does not belong to the repository must fail
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
		repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		token := getUserToken(t, repoOwner.Name, auth_model.AccessTokenScopeWriteIssue)
		req := NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/issues/comments/%d", repoOwner.Name, repo.Name, comment.ID).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	token := getUserToken(t, repoOwner.Name, auth_model.AccessTokenScopeWriteIssue)
	req := NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/issues/comments/%d", repoOwner.Name, repo.Name, comment.ID).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	unittest.AssertNotExistsBean(t, &issues_model.Comment{ID: comment.ID})
}

func TestAPIListIssueTimeline(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// load comment
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	// make request
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/%d/timeline", repoOwner.Name, repo.Name, issue.Index)
	resp := MakeRequest(t, req, http.StatusOK)

	// check if lens of list returned by API and
	// lists extracted directly from DB are the same
	var comments []*api.TimelineComment
	DecodeJSON(t, resp, &comments)
	expectedCount := unittest.GetCount(t, &issues_model.Comment{IssueID: issue.ID})
	assert.Len(t, comments, expectedCount)
}
