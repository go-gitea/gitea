// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers"
	repo_service "code.gitea.io/gitea/services/repository"
	"github.com/unknwon/com"
)

const (
	tplRepos          base.TplName = "admin/repo/list"
	tplUnadoptedRepos base.TplName = "admin/repo/unadopted"
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
	log.Trace("Repository deleted: %s", repo.FullName())

	ctx.Flash.Success(ctx.Tr("repo.settings.deletion_success"))
	ctx.JSON(200, map[string]interface{}{
		"redirect": setting.AppSubURL + "/admin/repos?page=" + ctx.Query("page") + "&sort=" + ctx.Query("sort"),
	})
}

// UnadoptedRepos lists the unadopted repositories
func UnadoptedRepos(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.repositories")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminRepositories"] = true

	opts := models.ListOptions{
		PageSize: setting.UI.Admin.UserPagingNum,
		Page:     ctx.QueryInt("page"),
	}

	if opts.Page <= 0 {
		opts.Page = 1
	}

	doSearch := ctx.QueryBool("search")

	ctx.Data["search"] = doSearch
	q := ctx.Query("q")

	if !doSearch {
		pager := context.NewPagination(0, opts.PageSize, opts.Page, 5)
		pager.SetDefaultParams(ctx)
		ctx.Data["Page"] = pager
		ctx.HTML(200, tplUnadoptedRepos)
		return
	}

	ctx.Data["Keyword"] = q
	repoNames, count, err := repository.ListUnadoptedRepositories(q, &opts)
	if err != nil {
		ctx.ServerError("ListUnadoptedRepositories", err)
	}
	ctx.Data["Dirs"] = repoNames
	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager
	ctx.HTML(200, tplUnadoptedRepos)
}

// AdoptOrDeleteRepository adopts or deletes a repository
func AdoptOrDeleteRepository(ctx *context.Context) {
	dir := ctx.Query("id")
	action := ctx.Query("action")
	dirSplit := strings.SplitN(dir, "/", 2)
	if len(dirSplit) != 2 {
		ctx.Redirect(setting.AppSubURL + "/admin/repos")
		return
	}

	ctxUser, err := models.GetUserByName(dirSplit[0])
	if err != nil {
		if models.IsErrUserNotExist(err) {
			log.Debug("User does not exist: %s", dirSplit[0])
			ctx.Redirect(setting.AppSubURL + "/admin/repos")
			return
		}
		ctx.ServerError("GetUserByName", err)
		return
	}

	repoName := dirSplit[1]

	// check not a repo
	if has, err := models.IsRepositoryExist(ctxUser, repoName); err != nil {
		ctx.ServerError("IsRepositoryExist", err)
		return
	} else if has || !com.IsDir(models.RepoPath(ctxUser.Name, repoName)) {
		// Fallthrough to failure mode
	} else if action == "adopt" {
		if _, err := repository.AdoptRepository(ctx.User, ctxUser, models.CreateRepoOptions{
			Name:      dirSplit[1],
			IsPrivate: true,
		}); err != nil {
			ctx.ServerError("repository.AdoptRepository", err)
			return
		}
		ctx.Flash.Success(ctx.Tr("repo.adopt_preexisting_success", dir))
	} else if action == "delete" {
		if err := repository.DeleteUnadoptedRepository(ctx.User, ctxUser, dirSplit[1]); err != nil {
			ctx.ServerError("repository.AdoptRepository", err)
			return
		}
		ctx.Flash.Success(ctx.Tr("repo.delete_preexisting_success", dir))
	}
	ctx.Redirect(setting.AppSubURL + "/admin/repos/unadopted")
}
