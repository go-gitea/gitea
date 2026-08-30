// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A public-only token must not be able to read a cross-repo PR's head-repository metadata or
// latest commit SHA when that head repository is private -- see GHSA-m78w-jjjx-gp8r. The base
// repository (and the PR itself) stays public and visible either way; only Head.Repository/Head.Sha
// must be hidden for the restricted token.
func TestAPIGetPullRequestPublicOnlyToken(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		org26 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 26})
		pr := createOutdatedPR(t, user, org26)
		require.NoError(t, pr.LoadBaseRepo(t.Context()))
		require.NoError(t, pr.LoadHeadRepo(t.Context()))

		require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(t.Context(),
			&repo_model.Repository{ID: pr.HeadRepo.ID, IsPrivate: true}, "is_private"))

		prURL := fmt.Sprintf("/api/v1/repos/%s/pulls/%d", pr.BaseRepo.FullName(), pr.Index)

		fullToken := getUserToken(t, user.Name, auth_model.AccessTokenScopeReadRepository)
		resp := MakeRequest(t, NewRequest(t, "GET", prURL).AddTokenAuth(fullToken), http.StatusOK)
		fullResp := DecodeJSON(t, resp, &api.PullRequest{})
		require.NotNil(t, fullResp.Head.Repository, "full token should still see the private head repo")
		assert.NotEmpty(t, fullResp.Head.Sha, "full token should still see the head commit SHA")

		publicOnlyToken := getUserToken(t, user.Name, auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly)
		resp = MakeRequest(t, NewRequest(t, "GET", prURL).AddTokenAuth(publicOnlyToken), http.StatusOK)
		publicResp := DecodeJSON(t, resp, &api.PullRequest{})
		assert.Nil(t, publicResp.Head.Repository, "public-only token must not see the private head repo")
		assert.Empty(t, publicResp.Head.Sha, "public-only token must not see the head commit SHA")
	})
}

func TestAPIListPullRequestsPublicOnlyToken(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		org26 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 26})
		pr := createOutdatedPR(t, user, org26)
		require.NoError(t, pr.LoadBaseRepo(t.Context()))
		require.NoError(t, pr.LoadHeadRepo(t.Context()))

		require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(t.Context(),
			&repo_model.Repository{ID: pr.HeadRepo.ID, IsPrivate: true}, "is_private"))

		listURL := fmt.Sprintf("/api/v1/repos/%s/pulls?state=all", pr.BaseRepo.FullName())

		publicOnlyToken := getUserToken(t, user.Name, auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly)
		resp := MakeRequest(t, NewRequest(t, "GET", listURL).AddTokenAuth(publicOnlyToken), http.StatusOK)
		var prs []*api.PullRequest
		DecodeJSON(t, resp, &prs)

		require.NotEmpty(t, prs)
		found := false
		for _, p := range prs {
			if p.Index == pr.Index {
				found = true
				assert.Nil(t, p.Head.Repository, "public-only token must not see the private head repo in list results")
				assert.Empty(t, p.Head.Sha, "public-only token must not see the head commit SHA in list results")
			}
		}
		assert.True(t, found, "expected PR to appear in the list")
	})
}
