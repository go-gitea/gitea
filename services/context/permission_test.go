// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context_test

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	module_auth "gitea.dev/modules/auth"
	gitea_context "gitea.dev/services/context"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckRepoScopedTokenForCodespaceToken(t *testing.T) {
	scope, err := auth_model.AccessTokenScope("write:repository").Normalize()
	require.NoError(t, err)

	for _, tc := range []struct {
		name     string
		request  string
		repo     *repo_model.Repository
		snapshot testCodespaceTokenSnapshot
		level    auth_model.AccessTokenScopeLevel
		status   int
	}{
		{
			name: "matched repo", request: "GET /user2/repo1.git/info/refs",
			repo: &repo_model.Repository{ID: 1}, snapshot: testCodespaceTokenSnapshot{repoID: 1},
			level: auth_model.Read,
		},
		{
			name: "private other repo read requires grant", request: "GET /user2/repo2.git/info/refs",
			repo: &repo_model.Repository{ID: 2, IsPrivate: true}, snapshot: testCodespaceTokenSnapshot{repoID: 1},
			level: auth_model.Read, status: http.StatusForbidden,
		},
		{
			name: "granted other repo read", request: "GET /user2/repo2.git/info/refs",
			repo:     &repo_model.Repository{ID: 2, IsPrivate: true},
			snapshot: testCodespaceTokenSnapshot{repoID: 1, grants: map[int64]map[unit.Type]perm.AccessMode{2: {unit.TypeCode: perm.AccessModeRead}}},
			level:    auth_model.Read,
		},
		{
			name: "public other repo read", request: "GET /user2/repo2.git/info/refs",
			repo: &repo_model.Repository{ID: 2, Owner: &user_model.User{}}, snapshot: testCodespaceTokenSnapshot{repoID: 1},
			level: auth_model.Read,
		},
		{
			name: "public other repo write", request: "POST /user2/repo2.git/git-receive-pack",
			repo: &repo_model.Repository{ID: 2}, snapshot: testCodespaceTokenSnapshot{repoID: 1},
			level: auth_model.Write, status: http.StatusForbidden,
		},
		{
			name: "missing source repository", request: "GET /user2/repo1.git/info/refs",
			repo:  &repo_model.Repository{ID: 1, IsPrivate: true},
			level: auth_model.Read, status: http.StatusForbidden,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := contexttest.MockContext(t, tc.request)
			ctx.Data["IsApiToken"] = true
			ctx.Doer = &user_model.User{ExtDoerData: user_model.NewTokenExtDoerData("codespace", 1, scope)}
			ctx.Data[module_auth.CodespaceTokenAuthDataKey] = tc.snapshot

			gitea_context.CheckRepoScopedToken(ctx, tc.repo, unit.TypeCode, tc.level)

			assert.Equal(t, tc.status, ctx.Resp.WrittenStatus())
		})
	}
}

type testCodespaceTokenSnapshot struct {
	repoID int64
	grants map[int64]map[unit.Type]perm.AccessMode
}

func (s testCodespaceTokenSnapshot) CodespaceTokenAllowsAnyRepository(repoID int64) bool {
	return repoID == s.repoID || len(s.grants[repoID]) > 0
}

func (s testCodespaceTokenSnapshot) CodespaceTokenAllowsRepository(repoID int64, unitType unit.Type, mode perm.AccessMode) bool {
	return repoID == s.repoID || s.grants[repoID][unitType] >= mode
}

func (s testCodespaceTokenSnapshot) CodespaceTokenRepoID() int64 {
	return s.repoID
}
