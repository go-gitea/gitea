// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	perm_model "gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePullReviewOnLockedPull(t *testing.T) {
	unittest.PrepareTestEnv(t)

	_, err := db.GetEngine(t.Context()).ID(3).Cols("is_locked").Update(&issues_model.Issue{IsLocked: true})
	require.NoError(t, err)

	request := func(mode perm_model.AccessMode) int {
		ctx, resp := contexttest.MockAPIContext(t, "user2/repo1")
		contexttest.LoadUser(t, ctx, 4)
		contexttest.LoadRepo(t, ctx, 1)
		contexttest.LoadGitRepo(t, ctx)
		defer ctx.Repo.GitRepo.Close()
		ctx.Repo.Permission = access_model.Permission{}
		ctx.Repo.Permission.SetUnitsWithDefaultAccessMode([]*repo_model.RepoUnit{{Type: unit.TypePullRequests}}, mode)
		ctx.SetPathParam("index", "3")
		web.SetForm(ctx, &api.CreatePullReviewOptions{Event: api.ReviewStateComment, Body: "review on locked pull"})
		CreatePullReview(ctx)
		return resp.Code
	}
	assert.Equal(t, http.StatusForbidden, request(perm_model.AccessModeRead))
	assert.Equal(t, http.StatusOK, request(perm_model.AccessModeWrite))
}
