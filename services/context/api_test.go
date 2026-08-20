// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestGenAPILinks(t *testing.T) {
	setting.AppURL = "http://localhost:3000/"
	kases := map[string][]string{
		"api/v1/repos/jerrykan/example-repo/issues?state=all": {
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=2&state=all>; rel="next"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=5&state=all>; rel="last"`,
		},
		"api/v1/repos/jerrykan/example-repo/issues?state=all&page=1": {
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=2&state=all>; rel="next"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=5&state=all>; rel="last"`,
		},
		"api/v1/repos/jerrykan/example-repo/issues?state=all&page=2": {
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=3&state=all>; rel="next"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=5&state=all>; rel="last"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=1&state=all>; rel="first"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=1&state=all>; rel="prev"`,
		},
		"api/v1/repos/jerrykan/example-repo/issues?state=all&page=5": {
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=1&state=all>; rel="first"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=4&state=all>; rel="prev"`,
		},
	}

	for req, response := range kases {
		u, err := url.Parse(setting.AppURL + req)
		assert.NoError(t, err)

		p := u.Query().Get("page")
		curPage, _ := strconv.Atoi(p)

		links := genAPILinks(u, 100, 20, curPage)

		assert.Equal(t, links, response)
	}
}

func TestAPIContextTokenCanAccessRepoForCodespaceToken(t *testing.T) {
	ctx := &APIContext{Base: &Base{RequestContext: reqctx.NewRequestContextForTest(t.Context())}}
	ctx.Req, _ = http.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/repos/user5/repo4", nil)
	ctx.GetData()[codespace_model.GiteaTokenAuthDataKey] = testCodespaceTokenSnapshot{repoID: 2}

	assert.True(t, ctx.TokenCanAccessRepo(&repo_model.Repository{ID: 2}))
	assert.False(t, ctx.TokenCanAccessRepo(&repo_model.Repository{ID: 3, IsPrivate: false}))
	assert.False(t, ctx.TokenCanAccessRepo(&repo_model.Repository{ID: 4, IsPrivate: true}))
	assert.False(t, ctx.TokenCanAccessRepo(nil))

	ctx.GetData()[codespace_model.GiteaTokenAuthDataKey] = testCodespaceTokenSnapshot{repoID: 0}
	assert.False(t, ctx.TokenCanAccessRepo(&repo_model.Repository{ID: 2, IsPrivate: false}))

	ctx.Req, _ = http.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/repos/user5/repo4", nil)
	assert.False(t, ctx.TokenCanAccessRepo(&repo_model.Repository{ID: 2, IsPrivate: false}))
}

func TestUseAnonymousForPublicCodespaceRead(t *testing.T) {
	ctx := &APIContext{Base: &Base{RequestContext: reqctx.NewRequestContextForTest(t.Context())}}
	ctx.Req, _ = http.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/repos/public/repo", nil)
	ctx.Doer = &user_model.User{ID: 2}
	ctx.IsSigned = true
	ctx.GetData()[codespace_model.GiteaTokenAuthDataKey] = testCodespaceTokenSnapshot{repoID: 1}
	ctx.GetData()["IsApiToken"] = true

	assert.True(t, ctx.UseAnonymousForPublicCodespaceRead(&repo_model.Repository{ID: 2, Owner: &user_model.User{}}))
	assert.Nil(t, ctx.Doer)
	assert.False(t, ctx.IsSigned)
	_, hasSnapshot := ctx.CodespaceTokenRepoID()
	assert.False(t, hasSnapshot)
}

type testCodespaceTokenSnapshot struct {
	repoID int64
}

func (s testCodespaceTokenSnapshot) CodespaceTokenRepoID() int64 {
	return s.repoID
}

func (s testCodespaceTokenSnapshot) CodespaceTokenAllowsAnyRepository(repoID int64) bool {
	return repoID == s.repoID
}

func (s testCodespaceTokenSnapshot) CodespaceTokenAllowsRepository(repoID int64, _ unit.Type, _ perm.AccessMode) bool {
	return repoID == s.repoID
}
