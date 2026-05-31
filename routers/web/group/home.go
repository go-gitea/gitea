// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"net/http"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	shared_group "gitea.dev/routers/web/shared/group"
	"gitea.dev/services/context"
)

const (
	tplGroupHome    = "group/home"
	tplGroupHomeOrg = "group/org_home"
)

func Home(ctx *context.Context) {
	org := ctx.ContextUser

	ctx.Data["PageIsUserProfile"] = true
	ctx.Data["Title"] = org.DisplayName()

	var orderBy db.SearchOrderBy
	sortOrder := ctx.FormString("sort")
	if _, ok := repo_model.OrderByFlatMap[sortOrder]; !ok {
		sortOrder = setting.UI.ExploreDefaultSort
	}
	ctx.Data["SortType"] = sortOrder
	orderBy = repo_model.OrderByFlatMap[sortOrder]

	keyword := ctx.FormTrim("q")
	ctx.Data["Keyword"] = keyword

	language := ctx.FormTrim("language")
	ctx.Data["Language"] = language

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	if ctx.RepoGroup.Group.Owner.IsIndividual() {
		pagingNum := setting.UI.User.RepoPagingNum
		_, numFollowers, err := user_model.GetUserFollowers(ctx, ctx.ContextUser, ctx.Doer, db.ListOptions{
			PageSize: pagingNum,
			Page:     page,
		})
		if err != nil {
			ctx.ServerError("GetUserFollowers", err)
			return
		}
		ctx.Data["NumFollowers"] = numFollowers
		_, numFollowing, err := user_model.GetUserFollowing(ctx, ctx.ContextUser, ctx.Doer, db.ListOptions{
			PageSize: pagingNum,
			Page:     page,
		})
		if err != nil {
			ctx.ServerError("GetUserFollowing", err)
			return
		}
		ctx.Data["NumFollowing"] = numFollowing
	}

	archived := ctx.FormOptionalBool("archived")
	ctx.Data["IsArchived"] = archived

	fork := ctx.FormOptionalBool("fork")
	ctx.Data["IsFork"] = fork

	mirror := ctx.FormOptionalBool("mirror")
	ctx.Data["IsMirror"] = mirror

	template := ctx.FormOptionalBool("template")
	ctx.Data["IsTemplate"] = template

	private := ctx.FormOptionalBool("private")
	ctx.Data["IsPrivate"] = private

	err := shared_group.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	ctx.Data["DisableNewPullMirrors"] = setting.Mirror.DisableNewPull
	ctx.Data["PageIsViewRepositories"] = true

	var (
		repos []*repo_model.Repository
		count int64
	)
	repos, count, err = repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			PageSize: setting.UI.User.RepoPagingNum,
			Page:     page,
		},
		Keyword:            keyword,
		OwnerID:            org.ID,
		OrderBy:            orderBy,
		Private:            ctx.IsSigned,
		Actor:              ctx.Doer,
		Language:           language,
		IncludeDescription: setting.UI.SearchRepoDescription,
		Archived:           archived,
		Fork:               fork,
		Mirror:             mirror,
		Template:           template,
		IsPrivate:          private,
		GroupID:            ctx.RepoGroup.Group.ID,
	})
	if err != nil {
		ctx.ServerError("SearchRepository", err)
		return
	}

	ctx.Data["Repos"] = repos
	ctx.Data["Total"] = count

	pager := context.NewPagination(count, setting.UI.User.RepoPagingNum, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager
	if ctx.RepoGroup.Group.Owner.IsIndividual() {
		ctx.HTML(http.StatusOK, tplGroupHome)
	} else {
		ctx.HTML(http.StatusOK, tplGroupHomeOrg)
	}
}
