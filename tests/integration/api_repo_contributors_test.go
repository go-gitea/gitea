// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIRepoContributors(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository)

	listContributors := func(t *testing.T, link string) ([]*api.Contributor, http.Header) {
		MakeRequest(t, NewRequest(t, "GET", link).AddTokenAuth(token), http.StatusAccepted)
		resp := MakeRequest(t, NewRequest(t, "GET", link).AddTokenAuth(token), http.StatusOK)
		return DecodeJSON(t, resp, []*api.Contributor{}), resp.Header()
	}

	t.Run("Users", func(t *testing.T) {
		contributors, header := listContributors(t, "/api/v1/repos/user2/repo16/contributors")
		assert.Equal(t, "2", header.Get("X-Total-Count"))
		if assert.Len(t, contributors, 2) {
			assert.Equal(t, "user2", contributors[0].Login)
			assert.Equal(t, &api.Contributor{
				Type:          "User",
				ID:            21,
				Login:         "user21",
				AvatarURL:     contributors[1].AvatarURL,
				HTMLURL:       contributors[1].HTMLURL,
				Contributions: 1,
				Additions:     1,
				Deletions:     1,
			}, contributors[1])
		}
	})

	t.Run("Anonymous", func(t *testing.T) {
		contributors, header := listContributors(t, "/api/v1/repos/user2/repo2/contributors?anon=1&per_page=1")
		assert.Equal(t, "3", header.Get("X-Total-Count"))
		assert.Contains(t, header.Get("Link"), `rel="next"`)
		assert.Equal(t, []*api.Contributor{{
			Type:          "Anonymous",
			Name:          "Ethan Koenig",
			Email:         "ethantkoenig@gmail.com",
			Contributions: 1,
			Additions:     3,
		}}, contributors)

		resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo2/contributors").AddTokenAuth(token), http.StatusOK)
		assert.Equal(t, "0", resp.Header().Get("X-Total-Count"))
	})

	t.Run("NoAccessOrEmpty", func(t *testing.T) {
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo16/contributors"), http.StatusNotFound)
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user30/repo50/contributors").AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/org3/repo21/contributors").AddTokenAuth(token), http.StatusNoContent)
	})
}
