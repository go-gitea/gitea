// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	codespace_model "gitea.dev/models/codespace"
	repo_model "gitea.dev/models/repo"
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
	assert.True(t, ctx.TokenCanAccessRepo(&repo_model.Repository{ID: 3, Owner: &user_model.User{}, IsPrivate: false}))
	assert.False(t, ctx.TokenCanAccessRepo(&repo_model.Repository{ID: 4, Owner: &user_model.User{}, IsPrivate: true}))
	assert.False(t, ctx.TokenCanAccessRepo(nil))
	assert.False(t, ctx.CodespaceTokenRepoBindingMismatch(&repo_model.Repository{ID: 2}))
	assert.True(t, ctx.CodespaceTokenRepoBindingMismatch(&repo_model.Repository{ID: 3}))
	assert.True(t, ctx.CodespaceTokenRepoBindingMismatch(nil))

	ctx.GetData()[codespace_model.GiteaTokenAuthDataKey] = testCodespaceTokenSnapshot{repoID: 0}
	assert.True(t, ctx.TokenCanAccessRepo(&repo_model.Repository{ID: 2, Owner: &user_model.User{}, IsPrivate: false}))
	assert.True(t, ctx.CodespaceTokenRepoBindingMismatch(&repo_model.Repository{ID: 2}))

	ctx.Req, _ = http.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/repos/user5/repo4", nil)
	assert.False(t, ctx.TokenCanAccessRepo(&repo_model.Repository{ID: 2, Owner: &user_model.User{}, IsPrivate: false}))
}

type testCodespaceTokenSnapshot struct {
	repoID int64
}

func (s testCodespaceTokenSnapshot) CodespaceTokenRepoID() int64 {
	return s.repoID
}
