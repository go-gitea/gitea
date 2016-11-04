// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"github.com/go-gitea/gitea/models"
	"github.com/go-gitea/gitea/modules/base"
	"github.com/go-gitea/gitea/modules/context"
	"github.com/go-gitea/gitea/modules/log"
	"github.com/go-gitea/gitea/modules/setting"
	"github.com/go-gitea/gitea/routers"
)

const (
	REPOS base.TplName = "admin/repo/list"
)

func Repos(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.repositories")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminRepositories"] = true

	routers.RenderRepoSearch(ctx, &routers.RepoSearchOptions{
		Counter:  models.CountRepositories,
		Ranger:   models.Repositories,
		Private:  true,
		PageSize: setting.UI.Admin.RepoPagingNum,
		OrderBy:  "id ASC",
		TplName:  REPOS,
	})
}

func DeleteRepo(ctx *context.Context) {
	repo, err := models.GetRepositoryByID(ctx.QueryInt64("id"))
	if err != nil {
		ctx.Handle(500, "GetRepositoryByID", err)
		return
	}

	if err := models.DeleteRepository(repo.MustOwner().ID, repo.ID); err != nil {
		ctx.Handle(500, "DeleteRepository", err)
		return
	}
	log.Trace("Repository deleted: %s/%s", repo.MustOwner().Name, repo.Name)

	ctx.Flash.Success(ctx.Tr("repo.settings.deletion_success"))
	ctx.JSON(200, map[string]interface{}{
		"redirect": setting.AppSubUrl + "/admin/repos?page=" + ctx.Query("page"),
	})
}
