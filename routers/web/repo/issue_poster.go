// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"slices"
	"strings"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	shared_user "gitea.dev/routers/web/shared/user"
	"gitea.dev/services/context"
)

func IssuePullPosters(ctx *context.Context) {
	isPullList := ctx.PathParam("type") == "pulls"
	issuePosters(ctx, isPullList)
}

func issuePosters(ctx *context.Context, isPullList bool) {
	repo := ctx.Repo.Repository
	search := strings.TrimSpace(ctx.FormString("q"))
	// a poster whose only issues are confidential ones hidden from the viewer is not listed
	visible := issues_model.NewVisibilityScopeForRepo(ctx.Doer, repo.ID, ctx.Repo.Permission).Cond()
	posters, err := repo_model.GetIssuePostersWithSearch(ctx, repo, isPullList, search, visible)
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
