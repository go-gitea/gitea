// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"
	"strconv"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/mailer/incoming"
)

func ResetRepoMailToRands(ctx *context.Context) {
	repoID, _ := strconv.ParseInt(ctx.PathParam("repo_id"), 10, 64)
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		ctx.ServerError("GetRepositoryByID", err)
		return
	}

	_, err = user_model.CreatRandsForRepository(ctx, ctx.Doer.ID, repo.ID, user_model.RepositoryRandsTypeNewIssue)
	if err != nil {
		ctx.ServerError("CreatRandsForRepository", err)
		return
	}

	_, url, err := incoming.GenerateMailToRepoURL(ctx, ctx.Doer, repo, user_model.RepositoryRandsTypeNewIssue)
	if err != nil {
		ctx.ServerError("GenerateMailToRepoURL", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{"url": url})
}
