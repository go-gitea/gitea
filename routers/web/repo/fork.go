// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"net/url"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	repo_service "code.gitea.io/gitea/services/repository"
)

const (
	tplFork templates.TplName = "repo/pulls/fork"
)

func getForkRepository(ctx *context.Context) *repo_model.Repository {
	forkRepo := ctx.Repo.Repository
	if ctx.Written() {
		return nil
	}

	if forkRepo.IsEmpty {
		log.Trace("Empty repository %-v", forkRepo)
		ctx.NotFound(nil)
		return nil
	}

	if err := forkRepo.LoadOwner(ctx); err != nil {
		ctx.ServerError("LoadOwner", err)
		return nil
	}

	ctx.Data["repo_name"] = forkRepo.Name
	ctx.Data["description"] = forkRepo.Description
	ctx.Data["IsPrivate"] = forkRepo.IsPrivate || forkRepo.Owner.Visibility == structs.VisibleTypePrivate
	canForkToUser := repository.CanUserForkBetweenOwners(forkRepo.OwnerID, ctx.Doer.ID) && !repo_model.HasForkedRepo(ctx, ctx.Doer.ID, forkRepo.ID)

	ctx.Data["ForkRepo"] = forkRepo

	ownedOrgs, err := organization.GetOrgsCanCreateRepoByUserID(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetOrgsCanCreateRepoByUserID", err)
		return nil
	}
	var orgs []*organization.Organization
	for _, org := range ownedOrgs {
		if forkRepo.OwnerID != org.ID && !repo_model.HasForkedRepo(ctx, org.ID, forkRepo.ID) {
			orgs = append(orgs, org)
		}
	}

	traverseParentRepo := forkRepo
	for {
		if !repository.CanUserForkBetweenOwners(ctx.Doer.ID, traverseParentRepo.OwnerID) {
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
		traverseParentRepo, err = repo_model.GetRepositoryByID(ctx, traverseParentRepo.ForkID)
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
	} else {
		ctx.Data["CanForkRepo"] = false
		ctx.Flash.Error(ctx.Tr("repo.fork_no_valid_owners"), true)
		return nil
	}

	branches, err := git_model.FindBranchNames(ctx, git_model.FindBranchOptions{
		RepoID:          ctx.Repo.Repository.ID,
		ListOptions:     db.ListOptionsAll,
		IsDeletedBranch: optional.Some(false),
		// Add it as the first option
		ExcludeBranchNames: []string{ctx.Repo.Repository.DefaultBranch},
	})
	if err != nil {
		ctx.ServerError("FindBranchNames", err)
		return nil
	}
	ctx.Data["Branches"] = append([]string{ctx.Repo.Repository.DefaultBranch}, branches...)

	return forkRepo
}

// Fork render repository fork page
func Fork(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("new_fork")

	if ctx.Doer.CanForkRepo() {
		ctx.Data["CanForkRepo"] = true
	} else {
		maxCreationLimit := ctx.Doer.MaxCreationLimit()
		msg := ctx.TrN(maxCreationLimit, "repo.form.reach_limit_of_creation_1", "repo.form.reach_limit_of_creation_n", maxCreationLimit)
		ctx.Flash.Error(msg, true)
	}

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
	ctx.Data["CanForkRepo"] = true

	ctxUser := checkContextUser(ctx, form.UID)
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
		if !repository.CanUserForkBetweenOwners(ctxUser.ID, traverseParentRepo.OwnerID) {
			ctx.RenderWithErr(ctx.Tr("repo.settings.new_owner_has_same_repo"), tplFork, &form)
			return
		}
		repo := repo_model.GetForkedRepo(ctx, ctxUser.ID, traverseParentRepo.ID)
		if repo != nil {
			ctx.Redirect(ctxUser.HomeLink() + "/" + url.PathEscape(repo.Name))
			return
		}
		if !traverseParentRepo.IsFork {
			break
		}
		traverseParentRepo, err = repo_model.GetRepositoryByID(ctx, traverseParentRepo.ForkID)
		if err != nil {
			ctx.ServerError("GetRepositoryByID", err)
			return
		}
	}

	// Check if user is allowed to create repo's on the organization.
	if ctxUser.IsOrganization() {
		isAllowedToFork, err := organization.OrgFromUser(ctxUser).CanCreateOrgRepo(ctx, ctx.Doer.ID)
		if err != nil {
			ctx.ServerError("CanCreateOrgRepo", err)
			return
		} else if !isAllowedToFork {
			ctx.HTTPError(http.StatusForbidden)
			return
		}
	}

	repo, err := repo_service.ForkRepository(ctx, ctx.Doer, ctxUser, repo_service.ForkRepoOptions{
		BaseRepo:     forkRepo,
		Name:         form.RepoName,
		Description:  form.Description,
		SingleBranch: form.ForkSingleBranch,
	})
	if err != nil {
		ctx.Data["Err_RepoName"] = true
		switch {
		case repo_model.IsErrReachLimitOfRepo(err):
			maxCreationLimit := ctxUser.MaxCreationLimit()
			msg := ctx.TrN(maxCreationLimit, "repo.form.reach_limit_of_creation_1", "repo.form.reach_limit_of_creation_n", maxCreationLimit)
			ctx.RenderWithErr(msg, tplFork, &form)
		case repo_model.IsErrRepoAlreadyExist(err):
			ctx.RenderWithErr(ctx.Tr("repo.settings.new_owner_has_same_repo"), tplFork, &form)
		case repo_model.IsErrRepoFilesAlreadyExist(err):
			switch {
			case ctx.IsUserSiteAdmin() || (setting.Repository.AllowAdoptionOfUnadoptedRepositories && setting.Repository.AllowDeleteOfUnadoptedRepositories):
				ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist.adopt_or_delete"), tplFork, form)
			case setting.Repository.AllowAdoptionOfUnadoptedRepositories:
				ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist.adopt"), tplFork, form)
			case setting.Repository.AllowDeleteOfUnadoptedRepositories:
				ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist.delete"), tplFork, form)
			default:
				ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist"), tplFork, form)
			}
		case db.IsErrNameReserved(err):
			ctx.RenderWithErr(ctx.Tr("repo.form.name_reserved", err.(db.ErrNameReserved).Name), tplFork, &form)
		case db.IsErrNamePatternNotAllowed(err):
			ctx.RenderWithErr(ctx.Tr("repo.form.name_pattern_not_allowed", err.(db.ErrNamePatternNotAllowed).Pattern), tplFork, &form)
		case errors.Is(err, user_model.ErrBlockedUser):
			ctx.RenderWithErr(ctx.Tr("repo.fork.blocked_user"), tplFork, form)
		default:
			ctx.ServerError("ForkPost", err)
		}
		return
	}

	log.Trace("Repository forked[%d]: %s/%s", forkRepo.ID, ctxUser.Name, repo.Name)
	ctx.Redirect(ctxUser.HomeLink() + "/" + url.PathEscape(repo.Name))
}
