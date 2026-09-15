// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git/gitcmd"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIReposGitRefs(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	for _, ref := range [...]string{
		"refs/heads/master", // Branch
		"refs/tags/v1.1",    // Tag
	} {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/%s", user.Name, ref).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)
	}
	// Test getting all refs
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/refs", user.Name).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)
	// Test getting non-existent refs
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/refs/heads/unknown", user.Name).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)

	getRefNames := func(refs []*api.Reference) []string {
		names := make([]string, 0, len(refs))
		for _, ref := range refs {
			names = append(names, ref.Ref)
		}
		return names
	}

	t.Run("FullRefNameReturnsObject", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/refs/heads/master", user.Name).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		ref := DecodeJSON(t, resp, api.Reference{})
		assert.Equal(t, "refs/heads/master", ref.Ref)
	})

	t.Run("PartialRefNameReturnsList", func(t *testing.T) {
		// "refs/heads/feature" is no reference, only "refs/heads/feature/1" starts with it
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/refs/heads/feature", user.Name).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		refs := DecodeJSON(t, resp, []*api.Reference{})
		assert.Equal(t, []string{"refs/heads/feature/1"}, getRefNames(refs))
	})

	t.Run("SharedPrefixDoesNotChangeResponse", func(t *testing.T) {
		// "master1" starts with "master", the response of "heads/master" must stay a single object
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user.ID, Name: "repo1"})
		_, _, runErr := gitcmd.NewCommand("update-ref").
			AddDynamicArguments("refs/heads/master1", "refs/heads/master").
			WithRepo(repo1).RunStdString(t.Context())
		require.NoError(t, runErr)

		req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/refs/heads/master", user.Name).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		ref := DecodeJSON(t, resp, api.Reference{})
		assert.Equal(t, "refs/heads/master", ref.Ref)

		req = NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/refs/heads/master1", user.Name).
			AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusOK)
		ref = DecodeJSON(t, resp, api.Reference{})
		assert.Equal(t, "refs/heads/master1", ref.Ref)

		// "heads/maste" is no reference, so both branches come back as a list
		req = NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/refs/heads/maste", user.Name).
			AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusOK)
		refs := DecodeJSON(t, resp, []*api.Reference{})
		assert.ElementsMatch(t, []string{"refs/heads/master", "refs/heads/master1"}, getRefNames(refs))
	})
}
