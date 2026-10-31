// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strconv"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPICommitComment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	const commitSHA = "65f1bf27bc3bf70f64657658635e66094edbcb4d"
	const repoURL = "/api/v1/repos/user2/repo1"
	const commitURL = repoURL + "/commits/" + commitSHA + "/comments"
	tokenFor := func(userID int64, scope auth_model.AccessTokenScope) string {
		t.Helper()
		token := &auth_model.AccessToken{UID: userID, Name: "commit-comments", Scope: scope}
		require.NoError(t, auth_model.NewAccessToken(t.Context(), token))
		return token.Token
	}
	posterToken := tokenFor(4, auth_model.AccessTokenScopeWriteRepository)
	otherToken := tokenFor(5, auth_model.AccessTokenScopeWriteRepository)
	ownerToken := tokenFor(2, auth_model.AccessTokenScopeWriteRepository)
	readToken := tokenFor(4, auth_model.AccessTokenScopeReadRepository)
	issueToken := tokenFor(4, auth_model.AccessTokenScopeWriteIssue)
	blockedToken := tokenFor(29, auth_model.AccessTokenScopeWriteRepository)

	MakeRequest(t, NewRequestWithJSON(t, "POST", commitURL, api.CreateCommitCommentOption{Body: "anonymous"}), http.StatusUnauthorized)
	MakeRequest(t, NewRequestWithJSON(t, "POST", commitURL, api.CreateCommitCommentOption{Body: "blocked"}).AddTokenAuth(blockedToken), http.StatusForbidden)
	for _, form := range []api.CreateCommitCommentOption{
		{},
		{Body: "missing line", Path: "README.md"},
		{Body: "missing path", Line: 1},
		{Body: "negative line", Path: "README.md", Line: -1},
		{Body: "invalid side", Path: "README.md", Line: 1, Side: "invalid"},
	} {
		MakeRequest(t, NewRequestWithJSON(t, "POST", commitURL, form).AddTokenAuth(posterToken), http.StatusUnprocessableEntity)
	}
	MakeRequest(t, NewRequest(t, "GET", repoURL+"/commits/0000000000000000000000000000000000000000/comments"), http.StatusNotFound)

	var comments []*api.CommitComment
	for _, form := range []api.CreateCommitCommentOption{
		{Body: "line comment", Path: "README.md", Line: 1},
		{Body: "old line comment", Path: "README.md", Line: 2, Side: "old"},
		{Body: "whole commit comment"},
	} {
		response := MakeRequest(t, NewRequestWithJSON(t, "POST", commitURL, form).AddTokenAuth(posterToken), http.StatusCreated)
		comment := DecodeJSON(t, response, &api.CommitComment{})
		comments = append(comments, comment)
		assert.Equal(t, form.Body, comment.Body)
		assert.Equal(t, commitSHA, comment.CommitID)
		assert.EqualValues(t, 4, comment.User.ID)
		assert.False(t, comment.Created.IsZero())
		assert.False(t, comment.Updated.IsZero())
		assert.Equal(t, setting.AppURL+"api/v1/repos/user2/repo1/comments/"+strconv.FormatInt(comment.ID, 10), comment.URL)
		assert.Nil(t, comment.Position)
		if form.Path == "" {
			assert.Nil(t, comment.Path)
			assert.Nil(t, comment.Line)
			assert.Empty(t, comment.Side)
			assert.Equal(t, setting.AppURL+"user2/repo1/commit/"+commitSHA+"#commitcomment-"+strconv.FormatInt(comment.ID, 10), comment.HTMLURL)
		} else {
			assert.Equal(t, new(form.Path), comment.Path)
			assert.Equal(t, new(form.Line), comment.Line)
			assert.Equal(t, util.IfZero(form.Side, "new"), comment.Side)
			assert.Equal(t, setting.AppURL+"user2/repo1/commit/"+commitSHA+"#r"+strconv.FormatInt(comment.ID, 10), comment.HTMLURL)
		}
	}
	for _, listURL := range []string{repoURL + "/comments", commitURL} {
		response := MakeRequest(t, NewRequest(t, "GET", listURL+"?limit=2").AddTokenAuth(readToken), http.StatusOK)
		assert.Equal(t, "3", response.Header().Get("X-Total-Count"))
		assert.Equal(t, comments[:2], DecodeJSON(t, response, []*api.CommitComment{}))
		response = MakeRequest(t, NewRequest(t, "GET", listURL+"?limit=2&page=2"), http.StatusOK)
		assert.Equal(t, comments[2:], DecodeJSON(t, response, []*api.CommitComment{}))
	}
	response := MakeRequest(t, NewRequest(t, "GET", repoURL+"/commits/5c050d3b6d2db231ab1f64e324f1b6b9a0b181c2/comments"), http.StatusOK)
	assert.Empty(t, DecodeJSON(t, response, []*api.CommitComment{}))
	commentURL := repoURL + "/comments/" + strconv.FormatInt(comments[0].ID, 10)
	response = MakeRequest(t, NewRequest(t, "GET", commentURL), http.StatusOK)
	assert.Equal(t, comments[0], DecodeJSON(t, response, &api.CommitComment{}))
	MakeRequest(t, NewRequestf(t, "GET", "/api/v1/repos/user5/repo4/comments/%d", comments[0].ID), http.StatusNotFound)
	MakeRequest(t, NewRequest(t, "GET", repoURL+"/comments/2"), http.StatusNotFound)
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo2/comments"), http.StatusNotFound)

	routes := []struct {
		method string
		url    string
	}{
		{"GET", repoURL + "/comments"},
		{"GET", commitURL},
		{"GET", commentURL},
		{"GET", commentURL + "/reactions"},
		{"POST", commitURL},
		{"PATCH", commentURL},
		{"DELETE", commentURL},
		{"POST", commentURL + "/reactions"},
		{"DELETE", commentURL + "/reactions"},
	}
	for _, route := range routes {
		MakeRequest(t, NewRequest(t, route.method, route.url).AddTokenAuth(issueToken), http.StatusForbidden)
		if route.method != "GET" {
			MakeRequest(t, NewRequest(t, route.method, route.url).AddTokenAuth(readToken), http.StatusForbidden)
		}
	}
	for _, method := range []string{"PATCH", "DELETE"} {
		MakeRequest(t, NewRequestWithJSON(t, method, commentURL, map[string]string{"body": "forbidden"}).AddTokenAuth(otherToken), http.StatusForbidden)
	}
	for _, edit := range []struct {
		token string
		body  string
	}{{posterToken, "poster edit"}, {ownerToken, "edited"}} {
		response = MakeRequest(t, NewRequestWithJSON(t, "PATCH", commentURL, map[string]string{"body": edit.body}).AddTokenAuth(edit.token), http.StatusOK)
		assert.Equal(t, edit.body, DecodeJSON(t, response, &api.CommitComment{}).Body)
	}
	response = MakeRequest(t, NewRequestWithJSON(t, "PATCH", commentURL, map[string]string{}).AddTokenAuth(posterToken), http.StatusOK)
	assert.Equal(t, "edited", DecodeJSON(t, response, &api.CommitComment{}).Body)
	MakeRequest(t, NewRequestWithJSON(t, "PATCH", commentURL, map[string]string{"body": ""}).AddTokenAuth(posterToken), http.StatusUnprocessableEntity)

	reactionURL := commentURL + "/reactions"
	for _, status := range []int{http.StatusCreated, http.StatusOK} {
		response = MakeRequest(t, NewRequestWithJSON(t, "POST", reactionURL, api.EditReactionOption{Reaction: "+1"}).AddTokenAuth(otherToken), status)
		reaction := DecodeJSON(t, response, &api.Reaction{})
		assert.Equal(t, "+1", reaction.Reaction)
		assert.EqualValues(t, 5, reaction.User.ID)
	}
	response = MakeRequest(t, NewRequest(t, "GET", reactionURL+"?limit=1"), http.StatusOK)
	assert.Len(t, DecodeJSON(t, response, []api.Reaction{}), 1)
	assert.Equal(t, "1", response.Header().Get("X-Total-Count"))
	MakeRequest(t, NewRequestWithJSON(t, "POST", reactionURL, api.EditReactionOption{Reaction: "invalid"}).AddTokenAuth(otherToken), http.StatusForbidden)
	MakeRequest(t, NewRequestWithJSON(t, "POST", reactionURL, api.EditReactionOption{Reaction: "+1"}).AddTokenAuth(blockedToken), http.StatusForbidden)
	MakeRequest(t, NewRequestWithJSON(t, "DELETE", reactionURL, api.EditReactionOption{Reaction: "+1"}).AddTokenAuth(otherToken), http.StatusNoContent)
	unittest.AssertNotExistsBean(t, &issues_model.Reaction{CommentID: comments[0].ID})

	func() {
		defer test.MockVariableValue(&setting.Repository.DisableCommitComments, true)()
		MakeRequest(t, NewRequestWithJSON(t, "POST", commitURL, api.CreateCommitCommentOption{Body: "disabled"}).AddTokenAuth(posterToken), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, "GET", commentURL), http.StatusOK)
	}()
	repo := &repo_model.Repository{ID: 1, IsArchived: true}
	require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(t.Context(), repo, "is_archived"))
	for _, route := range routes {
		if route.method == "GET" {
			MakeRequest(t, NewRequest(t, route.method, route.url), http.StatusOK)
		} else {
			MakeRequest(t, NewRequest(t, route.method, route.url).AddTokenAuth(posterToken), http.StatusLocked)
		}
	}
	repo.IsArchived = false
	require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(t.Context(), repo, "is_archived"))
	for index, token := range []string{ownerToken, posterToken, tokenFor(1, auth_model.AccessTokenScopeWriteRepository)} {
		commentURL := repoURL + "/comments/" + strconv.FormatInt(comments[index].ID, 10)
		MakeRequest(t, NewRequest(t, "DELETE", commentURL).AddTokenAuth(token), http.StatusNoContent)
		MakeRequest(t, NewRequest(t, "GET", commentURL), http.StatusNotFound)
	}
	require.NoError(t, db.DeleteBeans(t.Context(), &repo_model.RepoUnit{RepoID: 1, Type: unit.TypeCode}))
	MakeRequest(t, NewRequest(t, "GET", repoURL+"/comments"), http.StatusForbidden)
}
