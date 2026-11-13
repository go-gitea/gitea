// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/routers/web/explore"
	"gitea.dev/services/context"
	repo_service "gitea.dev/services/repository"
)

const (
	tplRepos templates.TplName = "admin/repo/list"
)

// Repos show all the repositories
func Repos(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.repositories")
	ctx.Data["PageIsAdminRepositories"] = true

	explore.RenderRepoSearch(ctx, &explore.RepoSearchOptions{
		Private:          true,
		PageSize:         setting.UI.Admin.RepoPagingNum,
		TplName:          tplRepos,
		OnlyShowRelevant: false,
	})
}

// DeleteRepo delete one repository
func DeleteRepo(ctx *context.Context) {
	repo, err := repo_model.GetRepositoryByID(ctx, ctx.FormInt64("id"))
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	if err := repo_service.DeleteRepository(ctx, ctx.Doer, repo, true); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	log.Trace("Repository deleted: %s", repo.FullName())

	ctx.Flash.Success(ctx.Tr("repo.settings.deletion_success"))
	ctx.JSONRedirect("")
}
