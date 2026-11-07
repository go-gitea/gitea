// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"slices"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	shared_user "gitea.dev/routers/web/shared/user"
	"gitea.dev/services/context"
)

func IssuePullPosters(ctx *context.Context) {
	// the issues route also admits readers of only pull requests, who may list those posters instead
	isPullList := ctx.PathParam("type") == "pulls" || !ctx.Repo.Permission.CanRead(unit.TypeIssues)
	issuePosters(ctx, isPullList)
}

func issuePosters(ctx *context.Context, isPullList bool) {
	repo := ctx.Repo.Repository
	search := strings.TrimSpace(ctx.FormString("q"))
	posters, err := repo_model.GetIssuePostersWithSearch(ctx, repo, isPullList, search)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}

	if search == "" && ctx.Doer != nil {
		// the returned posters slice only contains limited number of users,
		// to make the current user (doer) can quickly filter their own issues, always add doer to the posters slice
		if !slices.ContainsFunc(posters, func(user *user_model.User) bool { return user.ID == ctx.Doer.ID }) {
			posters = append(posters, ctx.Doer)
		}
	}
	ctx.JSON(http.StatusOK, shared_user.ToSearchUserResponse(ctx, ctx.Doer, posters))
}
