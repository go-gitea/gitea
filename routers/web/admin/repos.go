// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/web/explore"
	"code.gitea.io/gitea/services/context"
	repo_service "code.gitea.io/gitea/services/repository"
)

const (
	tplRepos          templates.TplName = "admin/repo/list"
	tplUnadoptedRepos templates.TplName = "admin/repo/unadopted"
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
		ctx.ServerError("GetRepositoryByID", err)
		return
	}

	if ctx.Repo != nil && ctx.Repo.GitRepo != nil && ctx.Repo.Repository != nil && ctx.Repo.Repository.ID == repo.ID {
		ctx.Repo.GitRepo.Close()
	}

	if err := repo_service.DeleteRepository(ctx, ctx.Doer, repo, true); err != nil {
		ctx.ServerError("DeleteRepository", err)
		return
	}
	log.Trace("Repository deleted: %s", repo.FullName())

	ctx.Flash.Success(ctx.Tr("repo.settings.deletion_success"))
	ctx.JSONRedirect(setting.AppSubURL + "/-/admin/repos?page=" + url.QueryEscape(ctx.FormString("page")) + "&sort=" + url.QueryEscape(ctx.FormString("sort")))
}

// UnadoptedRepos lists the unadopted repositories
func UnadoptedRepos(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.repositories")
	ctx.Data["PageIsAdminRepositories"] = true

	opts := db.ListOptions{
		PageSize: setting.UI.Admin.UserPagingNum,
		Page:     ctx.FormInt("page"),
	}

	if opts.Page <= 0 {
		opts.Page = 1
	}

	ctx.Data["CurrentPage"] = opts.Page

	doSearch := ctx.FormBool("search")

	ctx.Data["search"] = doSearch
	q := ctx.FormString("q")

	if !doSearch {
		pager := context.NewPagination(0, opts.PageSize, opts.Page, 5)
		pager.AddParamFromRequest(ctx.Req)
		ctx.Data["Page"] = pager
		ctx.HTML(http.StatusOK, tplUnadoptedRepos)
		return
	}

	ctx.Data["Keyword"] = q
	repoNames, count, err := repo_service.ListUnadoptedRepositories(ctx, q, &opts)
	if err != nil {
		ctx.ServerError("ListUnadoptedRepositories", err)
		return
	}
	ctx.Data["Dirs"] = repoNames
	pager := context.NewPagination(count, opts.PageSize, opts.Page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager
	ctx.HTML(http.StatusOK, tplUnadoptedRepos)
}

// AdoptOrDeleteRepository adopts or deletes a repository
func AdoptOrDeleteRepository(ctx *context.Context) {
	dir := ctx.FormString("id")
	action := ctx.FormString("action")
	page := ctx.FormString("page")
	q := ctx.FormString("q")

	dirSplit := strings.SplitN(dir, "/", 2)
	if len(dirSplit) != 2 {
		ctx.Redirect(setting.AppSubURL + "/-/admin/repos")
		return
	}

	ctxUser, err := user_model.GetUserByName(ctx, dirSplit[0])
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			log.Debug("User does not exist: %s", dirSplit[0])
			ctx.Redirect(setting.AppSubURL + "/-/admin/repos")
			return
		}
		ctx.ServerError("GetUserByName", err)
		return
	}

	repoName := dirSplit[1]

	// check not a repo
	has, err := repo_model.IsRepositoryModelExist(ctx, ctxUser, repoName)
	if err != nil {
		ctx.ServerError("IsRepositoryExist", err)
		return
	}
	isDir, err := util.IsDir(repo_model.RepoPath(ctxUser.Name, repoName))
	if err != nil {
		ctx.ServerError("IsDir", err)
		return
	}
	if has || !isDir {
		// Fallthrough to failure mode
	} else if action == "adopt" {
		if _, err := repo_service.AdoptRepository(ctx, ctx.Doer, ctxUser, repo_service.CreateRepoOptions{
			Name:      dirSplit[1],
			IsPrivate: true,
		}); err != nil {
			ctx.ServerError("repository.AdoptRepository", err)
			return
		}
		ctx.Flash.Success(ctx.Tr("repo.adopt_preexisting_success", dir))
	} else if action == "delete" {
		if err := repo_service.DeleteUnadoptedRepository(ctx, ctx.Doer, ctxUser, dirSplit[1]); err != nil {
			ctx.ServerError("repository.AdoptRepository", err)
			return
		}
		ctx.Flash.Success(ctx.Tr("repo.delete_preexisting_success", dir))
	}
	ctx.Redirect(setting.AppSubURL + "/-/admin/repos/unadopted?search=true&q=" + url.QueryEscape(q) + "&page=" + url.QueryEscape(page))
}
