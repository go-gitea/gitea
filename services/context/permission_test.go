// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context_test

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	gitea_context "gitea.dev/services/context"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckRepoScopedTokenForCodespaceToken(t *testing.T) {
	scope, err := auth_model.AccessTokenScope("write:repository").Normalize()
	require.NoError(t, err)

	t.Run("matched repo", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "GET /user2/repo1.git/info/refs")
		ctx.Data["IsApiToken"] = true
		ctx.Data["ApiTokenScope"] = scope
		ctx.Data[codespace_model.GiteaTokenAuthDataKey] = testCodespaceTokenSnapshot{repoID: 1}

		gitea_context.CheckRepoScopedToken(ctx, &repo_model.Repository{ID: 1}, unit.TypeCode, auth_model.Read)

		assert.Equal(t, 0, ctx.Resp.WrittenStatus())
	})

	t.Run("private other repo read requires grant", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "GET /user2/repo1.git/info/refs")
		ctx.Data["IsApiToken"] = true
		ctx.Data["ApiTokenScope"] = scope
		ctx.Data[codespace_model.GiteaTokenAuthDataKey] = testCodespaceTokenSnapshot{repoID: 1}

		gitea_context.CheckRepoScopedToken(ctx, &repo_model.Repository{ID: 2, IsPrivate: true}, unit.TypeCode, auth_model.Read)

		assert.Equal(t, http.StatusForbidden, ctx.Resp.WrittenStatus())
	})

	t.Run("granted other repo read", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "GET /user2/repo1.git/info/refs")
		ctx.Data["IsApiToken"] = true
		ctx.Data["ApiTokenScope"] = scope
		ctx.Data[codespace_model.GiteaTokenAuthDataKey] = testCodespaceTokenSnapshot{repoID: 1, grants: map[int64]map[unit.Type]perm.AccessMode{2: {unit.TypeCode: perm.AccessModeRead}}}

		gitea_context.CheckRepoScopedToken(ctx, &repo_model.Repository{ID: 2}, unit.TypeCode, auth_model.Read)

		assert.Equal(t, 0, ctx.Resp.WrittenStatus())
	})

	t.Run("public other repo read", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "GET /user2/repo1.git/info/refs")
		ctx.Data["IsApiToken"] = true
		ctx.Data["ApiTokenScope"] = scope
		ctx.Data[codespace_model.GiteaTokenAuthDataKey] = testCodespaceTokenSnapshot{repoID: 1}

		gitea_context.CheckRepoScopedToken(ctx, &repo_model.Repository{ID: 2, Owner: &user_model.User{}}, unit.TypeCode, auth_model.Read)

		assert.Equal(t, 0, ctx.Resp.WrittenStatus())
	})

	t.Run("public other repo write", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "POST /user2/repo1.git/git-receive-pack")
		ctx.Data["IsApiToken"] = true
		ctx.Data["ApiTokenScope"] = scope
		ctx.Data[codespace_model.GiteaTokenAuthDataKey] = testCodespaceTokenSnapshot{repoID: 1}

		gitea_context.CheckRepoScopedToken(ctx, &repo_model.Repository{ID: 2}, unit.TypeCode, auth_model.Write)

		assert.Equal(t, http.StatusForbidden, ctx.Resp.WrittenStatus())
	})

	t.Run("missing source repository", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "GET /user2/repo1.git/info/refs")
		ctx.Data["IsApiToken"] = true
		ctx.Data["ApiTokenScope"] = scope
		ctx.Data[codespace_model.GiteaTokenAuthDataKey] = testCodespaceTokenSnapshot{repoID: 0}

		gitea_context.CheckRepoScopedToken(ctx, &repo_model.Repository{ID: 1, IsPrivate: true}, unit.TypeCode, auth_model.Read)

		assert.Equal(t, http.StatusForbidden, ctx.Resp.WrittenStatus())
	})
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
