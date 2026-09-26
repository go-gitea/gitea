// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRestrictedUserLimitedOwner(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})
	owner.Visibility = api.VisibleTypeLimited
	require.NoError(t, user_model.UpdateUserCols(t.Context(), owner, "visibility"))

	restrictedToken := getUserToken(t, "user29", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadIssue, auth_model.AccessTokenScopeReadOrganization)
	for _, path := range []string{
		"/api/v1/users/user2/activities/feeds",
		"/api/v1/users/user2/heatmap",
		"/api/v1/users/user2/keys",
		"/api/v1/users/user2/gpg_keys",
		"/api/v1/users/user2/orgs",
	} {
		req := NewRequest(t, "GET", path).AddTokenAuth(restrictedToken)
		MakeRequest(t, req, http.StatusNotFound)
	}

	issueSearch := url.URL{Path: "/api/v1/repos/issues/search"}
	issueSearch.RawQuery = url.Values{"owner": {"user2"}}.Encode()
	req := NewRequest(t, "GET", issueSearch.String()).AddTokenAuth(restrictedToken)
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Empty(t, DecodeJSON(t, resp, []*api.Issue{}))

	issueSearch.RawQuery = url.Values{"limit": {"100"}, "type": {"issues"}}.Encode()
	req = NewRequest(t, "GET", issueSearch.String()).AddTokenAuth(restrictedToken)
	resp = MakeRequest(t, req, http.StatusOK)
	issues := DecodeJSON(t, resp, []*api.Issue{})
	require.NotEmpty(t, issues)
	for _, issue := range issues {
		assert.NotEqual(t, owner.Name, issue.Repo.Owner)
	}

	restrictedSession := loginUser(t, "user29")
	req = NewRequest(t, "GET", "/issues/search?owner=user2")
	resp = restrictedSession.MakeRequest(t, req, http.StatusOK)
	assert.Empty(t, DecodeJSON(t, resp, []*api.Issue{}))

	viewerToken := getUserToken(t, "user4", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadIssue, auth_model.AccessTokenScopeReadOrganization)
	for _, path := range []string{
		"/api/v1/users/user2/activities/feeds",
		"/api/v1/users/user2/heatmap",
		"/api/v1/users/user2/keys",
		"/api/v1/users/user2/gpg_keys",
		"/api/v1/users/user2/orgs",
	} {
		req := NewRequest(t, "GET", path).AddTokenAuth(viewerToken)
		MakeRequest(t, req, http.StatusOK)
	}

	issueSearch.RawQuery = url.Values{"owner": {"user2"}}.Encode()
	req = NewRequest(t, "GET", issueSearch.String()).AddTokenAuth(viewerToken)
	resp = MakeRequest(t, req, http.StatusOK)
	assert.NotEmpty(t, DecodeJSON(t, resp, []*api.Issue{}))
}
