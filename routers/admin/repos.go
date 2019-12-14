// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers"
	repo_service "code.gitea.io/gitea/services/repository"
)

const (
	tplRepos base.TplName = "admin/repo/list"
)

// Repos show all the repositories
func Repos(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.repositories")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminRepositories"] = true

	routers.RenderRepoSearch(ctx, &routers.RepoSearchOptions{
		Private:  true,
		PageSize: setting.UI.Admin.RepoPagingNum,
		TplName:  tplRepos,
	})
}

// DeleteRepo delete one repository
func DeleteRepo(ctx *context.Context) {
	repo, err := models.GetRepositoryByID(ctx.QueryInt64("id"))
	if err != nil {
		ctx.ServerError("GetRepositoryByID", err)
		return
	}

	if err := repo_service.DeleteRepository(ctx.User, repo); err != nil {
		ctx.ServerError("DeleteRepository", err)
		return
	}
	log.Trace("Repository deleted: %s/%s", repo.MustOwner().Name, repo.Name)

	ctx.Flash.Success(ctx.Tr("repo.settings.deletion_success"))
	ctx.JSON(200, map[string]interface{}{
		"redirect": setting.AppSubURL + "/admin/repos?page=" + ctx.Query("page") + "&sort=" + ctx.Query("sort"),
	})
}
