// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"net/url"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/repo/common"
	"code.gitea.io/gitea/services/forms"
	repo_service "code.gitea.io/gitea/services/repository"
)

const (
	tplForks base.TplName = "repo/forks"
	tplFork  base.TplName = "repo/pulls/fork"
)

// Forks render repository's forked users
func Forks(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repos.forks")

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	pager := context.NewPagination(ctx.Repo.Repository.NumForks, setting.ItemsPerPage, page, 5)
	ctx.Data["Page"] = pager

	forks, err := repo_model.GetForks(ctx.Repo.Repository, db.ListOptions{
		Page:     pager.Paginater.Current(),
		PageSize: setting.ItemsPerPage,
	})
	if err != nil {
		ctx.ServerError("GetForks", err)
		return
	}

	for _, fork := range forks {
		if err = fork.GetOwner(ctx); err != nil {
			ctx.ServerError("GetOwner", err)
			return
		}
	}

	ctx.Data["Forks"] = forks

	ctx.HTML(http.StatusOK, tplForks)
}

func getForkRepository(ctx *context.Context) *repo_model.Repository {
	forkRepo := getRepository(ctx, ctx.ParamsInt64(":repoid"))
	if ctx.Written() {
		return nil
	}

	if forkRepo.IsEmpty {
		log.Trace("Empty repository %-v", forkRepo)
		ctx.NotFound("getForkRepository", nil)
		return nil
	}

	if err := forkRepo.GetOwner(ctx); err != nil {
		ctx.ServerError("GetOwner", err)
		return nil
	}

	ctx.Data["repo_name"] = forkRepo.Name
	ctx.Data["description"] = forkRepo.Description
	ctx.Data["IsPrivate"] = forkRepo.IsPrivate || forkRepo.Owner.Visibility == structs.VisibleTypePrivate
	canForkToUser := forkRepo.OwnerID != ctx.Doer.ID && !repo_model.HasForkedRepo(ctx.Doer.ID, forkRepo.ID)

	ctx.Data["ForkRepo"] = forkRepo

	ownedOrgs, err := organization.GetOrgsCanCreateRepoByUserID(ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetOrgsCanCreateRepoByUserID", err)
		return nil
	}
	var orgs []*organization.Organization
	for _, org := range ownedOrgs {
		if forkRepo.OwnerID != org.ID && !repo_model.HasForkedRepo(org.ID, forkRepo.ID) {
			orgs = append(orgs, org)
		}
	}

	traverseParentRepo := forkRepo
	for {
		if ctx.Doer.ID == traverseParentRepo.OwnerID {
			canForkToUser = false
		} else {
			for i, org := range orgs {
				if org.ID == traverseParentRepo.OwnerID {
					orgs = append(orgs[:i], orgs[i+1:]...)
					break
				}
			}
		}

		if !traverseParentRepo.IsFork {
			break
		}
		traverseParentRepo, err = repo_model.GetRepositoryByID(traverseParentRepo.ForkID)
		if err != nil {
			ctx.ServerError("GetRepositoryByID", err)
			return nil
		}
	}

	ctx.Data["CanForkToUser"] = canForkToUser
	ctx.Data["Orgs"] = orgs

	if canForkToUser {
		ctx.Data["ContextUser"] = ctx.Doer
	} else if len(orgs) > 0 {
		ctx.Data["ContextUser"] = orgs[0]
	}

	return forkRepo
}

// Fork render repository fork page
func Fork(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("new_fork")

	getForkRepository(ctx)
	if ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplFork)
}

// ForkPost response for forking a repository
func ForkPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateRepoForm)
	ctx.Data["Title"] = ctx.Tr("new_fork")

	ctxUser := common.CheckContextUser(ctx, form.UID)
	if ctx.Written() {
		return
	}

	forkRepo := getForkRepository(ctx)
	if ctx.Written() {
		return
	}

	ctx.Data["ContextUser"] = ctxUser

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplFork)
		return
	}

	var err error
	traverseParentRepo := forkRepo
	for {
		if ctxUser.ID == traverseParentRepo.OwnerID {
			ctx.RenderWithErr(ctx.Tr("repo.settings.new_owner_has_same_repo"), tplFork, &form)
			return
		}
		repo := repo_model.GetForkedRepo(ctxUser.ID, traverseParentRepo.ID)
		if repo != nil {
			ctx.Redirect(ctxUser.HomeLink() + "/" + url.PathEscape(repo.Name))
			return
		}
		if !traverseParentRepo.IsFork {
			break
		}
		traverseParentRepo, err = repo_model.GetRepositoryByID(traverseParentRepo.ForkID)
		if err != nil {
			ctx.ServerError("GetRepositoryByID", err)
			return
		}
	}

	// Check if user is allowed to create repo's on the organization.
	if ctxUser.IsOrganization() {
		isAllowedToFork, err := organization.OrgFromUser(ctxUser).CanCreateOrgRepo(ctx.Doer.ID)
		if err != nil {
			ctx.ServerError("CanCreateOrgRepo", err)
			return
		} else if !isAllowedToFork {
			ctx.Error(http.StatusForbidden)
			return
		}
	}

	repo, err := repo_service.ForkRepository(ctx, ctx.Doer, ctxUser, repo_service.ForkRepoOptions{
		BaseRepo:    forkRepo,
		Name:        form.RepoName,
		Description: form.Description,
	})
	if err != nil {
		ctx.Data["Err_RepoName"] = true
		switch {
		case repo_model.IsErrRepoAlreadyExist(err):
			ctx.RenderWithErr(ctx.Tr("repo.settings.new_owner_has_same_repo"), tplFork, &form)
		case db.IsErrNameReserved(err):
			ctx.RenderWithErr(ctx.Tr("repo.form.name_reserved", err.(db.ErrNameReserved).Name), tplFork, &form)
		case db.IsErrNamePatternNotAllowed(err):
			ctx.RenderWithErr(ctx.Tr("repo.form.name_pattern_not_allowed", err.(db.ErrNamePatternNotAllowed).Pattern), tplFork, &form)
		default:
			ctx.ServerError("ForkPost", err)
		}
		return
	}

	log.Trace("Repository forked[%d]: %s/%s", forkRepo.ID, ctxUser.Name, repo.Name)
	ctx.Redirect(ctxUser.HomeLink() + "/" + url.PathEscape(repo.Name))
}
