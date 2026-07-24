// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context_test

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	codespace_model "gitea.dev/models/codespace"
	repo_model "gitea.dev/models/repo"
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

		gitea_context.CheckRepoScopedToken(ctx, &repo_model.Repository{ID: 1}, auth_model.Read)

		assert.Equal(t, 0, ctx.Resp.WrittenStatus())
	})

	t.Run("public other repo read", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "GET /user2/repo1.git/info/refs")
		ctx.Data["IsApiToken"] = true
		ctx.Data["ApiTokenScope"] = scope
		ctx.Data[codespace_model.GiteaTokenAuthDataKey] = testCodespaceTokenSnapshot{repoID: 1}

		gitea_context.CheckRepoScopedToken(ctx, &repo_model.Repository{ID: 2, Owner: &user_model.User{}}, auth_model.Read)

		assert.Equal(t, 0, ctx.Resp.WrittenStatus())
	})

	t.Run("public other repo write", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "POST /user2/repo1.git/git-receive-pack")
		ctx.Data["IsApiToken"] = true
		ctx.Data["ApiTokenScope"] = scope
		ctx.Data[codespace_model.GiteaTokenAuthDataKey] = testCodespaceTokenSnapshot{repoID: 1}

		gitea_context.CheckRepoScopedToken(ctx, &repo_model.Repository{ID: 2, Owner: &user_model.User{}}, auth_model.Write)

		assert.Equal(t, http.StatusForbidden, ctx.Resp.WrittenStatus())
	})

	t.Run("private other repo read", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "GET /user2/repo1.git/info/refs")
		ctx.Data["IsApiToken"] = true
		ctx.Data["ApiTokenScope"] = scope
		ctx.Data[codespace_model.GiteaTokenAuthDataKey] = testCodespaceTokenSnapshot{repoID: 1}

		gitea_context.CheckRepoScopedToken(ctx, &repo_model.Repository{ID: 2, Owner: &user_model.User{}, IsPrivate: true}, auth_model.Read)

		assert.Equal(t, http.StatusForbidden, ctx.Resp.WrittenStatus())
	})

	t.Run("empty repo binding", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "GET /user2/repo1.git/info/refs")
		ctx.Data["IsApiToken"] = true
		ctx.Data["ApiTokenScope"] = scope
		ctx.Data[codespace_model.GiteaTokenAuthDataKey] = testCodespaceTokenSnapshot{repoID: 0}

		gitea_context.CheckRepoScopedToken(ctx, &repo_model.Repository{ID: 1, Owner: &user_model.User{}, IsPrivate: true}, auth_model.Read)

		assert.Equal(t, http.StatusForbidden, ctx.Resp.WrittenStatus())
	})
}

type testCodespaceTokenSnapshot struct {
	repoID int64
}

func (s testCodespaceTokenSnapshot) CodespaceTokenRepoID() int64 {
	return s.repoID
}
