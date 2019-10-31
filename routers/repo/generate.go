// Copyright 2019 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	repo_service "code.gitea.io/gitea/services/repository"
)

const (
	tplGenerate    base.TplName = "repo/pulls/generate"
)

func getTemplateRepository(ctx *context.Context) *models.Repository {
	templateRepo, err := models.GetRepositoryByID(ctx.ParamsInt64(":repoid"))
	if err != nil {
		if models.IsErrRepoNotExist(err) {
			ctx.NotFound("GetRepositoryByID", nil)
		} else {
			ctx.ServerError("GetRepositoryByID", err)
		}
		return nil
	}

	perm, err := models.GetUserRepoPermission(templateRepo, ctx.User)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return nil
	}

	if templateRepo.IsEmpty || !perm.CanRead(models.UnitTypeCode) {
		if log.IsTrace() {
			if templateRepo.IsEmpty {
				log.Trace("Empty fork repository %-v", templateRepo)
			} else {
				log.Trace("Permission Denied: User %-v cannot read %-v of forkRepo %-v\n"+
					"User in forkRepo has Permissions: %-+v",
					ctx.User,
					models.UnitTypeCode,
					ctx.Repo,
					perm)
			}
		}
		ctx.NotFound("getForkRepository", nil)
		return nil
	}

	ctx.Data["repo_name"] = templateRepo.Name
	ctx.Data["description"] = templateRepo.Description
	ctx.Data["private"] = templateRepo.IsPrivate
	ctx.Data["IsForcedPrivate"] = setting.Repository.ForcePrivate

	if err = templateRepo.GetOwner(); err != nil {
		ctx.ServerError("GetOwner", err)
		return nil
	}
	ctx.Data["GenerateFrom"] = templateRepo.Owner.Name + "/" + templateRepo.Name
	ctx.Data["GenerateFromOwnerID"] = templateRepo.Owner.ID

	if err := ctx.User.GetOwnedOrganizations(); err != nil {
		ctx.ServerError("GetOwnedOrganizations", err)
		return nil
	}

	ctx.Data["Orgs"] = ctx.User.OwnedOrgs

	ctx.Data["ContextUser"] = ctx.User

	return templateRepo
}

// TemplateGenerate render repository template generate page
func TemplateGenerate(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("template_generate")

	getTemplateRepository(ctx)
	if ctx.Written() {
		return
	}

	ctx.HTML(200, tplGenerate)
}

// TemplateGeneratePost response for generating a repository template
func TemplateGeneratePost(ctx *context.Context, form auth.CreateRepoForm) {
	ctx.Data["Title"] = ctx.Tr("template_generate")

	ctxUser := checkContextUser(ctx, form.UID)
	if ctx.Written() {
		return
	}

	templateRepo := getTemplateRepository(ctx)
	if ctx.Written() {
		return
	}

	ctx.Data["ContextUser"] = ctxUser

	if ctx.HasError() {
		ctx.HTML(200, tplGenerate)
		return
	}

	// Check ownership of organization.
	if ctxUser.IsOrganization() {
		isOwner, err := ctxUser.IsOwnedBy(ctx.User.ID)
		if err != nil {
			ctx.ServerError("IsOwnedBy", err)
			return
		} else if !isOwner {
			ctx.Error(403)
			return
		}
	}

	private := form.Private || setting.Repository.ForcePrivate
	repo, err := repo_service.GenerateRepository(ctx.User, ctxUser, templateRepo, form.RepoName, form.Description, private)
	if err != nil {
		ctx.Data["Err_RepoName"] = true
		switch {
		case models.IsErrRepoAlreadyExist(err):
			ctx.RenderWithErr(ctx.Tr("repo.settings.new_owner_has_same_repo"), tplGenerate, &form)
		case models.IsErrNameReserved(err):
			ctx.RenderWithErr(ctx.Tr("repo.form.name_reserved", err.(models.ErrNameReserved).Name), tplGenerate, &form)
		case models.IsErrNamePatternNotAllowed(err):
			ctx.RenderWithErr(ctx.Tr("repo.form.name_pattern_not_allowed", err.(models.ErrNamePatternNotAllowed).Pattern), tplGenerate, &form)
		default:
			ctx.ServerError("TemplateGeneratePost", err)
		}
		return
	}

	log.Trace("Repository generate[%d]: %s/%s", templateRepo.ID, ctxUser.Name, repo.Name)
	ctx.Redirect(setting.AppSubURL + "/" + ctxUser.Name + "/" + repo.Name)
}
