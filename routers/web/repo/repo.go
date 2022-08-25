// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/repo/common"
	"code.gitea.io/gitea/services/forms"
	repo_service "code.gitea.io/gitea/services/repository"
)

const (
	tplCreate   base.TplName = "repo/create"
	tplWatchers base.TplName = "repo/watchers"
)

// MustBeNotEmpty render when a repo is a empty git dir
func MustBeNotEmpty(ctx *context.Context) {
	if ctx.Repo.Repository.IsEmpty {
		ctx.NotFound("MustBeNotEmpty", nil)
	}
}

// MustBeEditable check that repo can be edited
func MustBeEditable(ctx *context.Context) {
	if !ctx.Repo.Repository.CanEnableEditor() || ctx.Repo.IsViewCommit {
		ctx.NotFound("", nil)
		return
	}
}

// MustBeAbleToUpload check that repo can be uploaded to
func MustBeAbleToUpload(ctx *context.Context) {
	if !setting.Repository.Upload.Enabled {
		ctx.NotFound("", nil)
	}
}

func getRepoPrivate(ctx *context.Context) bool {
	switch strings.ToLower(setting.Repository.DefaultPrivate) {
	case setting.RepoCreatingLastUserVisibility:
		return ctx.Doer.LastRepoVisibility
	case setting.RepoCreatingPrivate:
		return true
	case setting.RepoCreatingPublic:
		return false
	default:
		return ctx.Doer.LastRepoVisibility
	}
}

// Create render creating repository page
func Create(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("new_repo")

	// Give default value for template to render.
	ctx.Data["Gitignores"] = repo_module.Gitignores
	ctx.Data["LabelTemplates"] = repo_module.LabelTemplates
	ctx.Data["Licenses"] = repo_module.Licenses
	ctx.Data["Readmes"] = repo_module.Readmes
	ctx.Data["readme"] = "Default"
	ctx.Data["private"] = getRepoPrivate(ctx)
	ctx.Data["IsForcedPrivate"] = setting.Repository.ForcePrivate
	ctx.Data["default_branch"] = setting.Repository.DefaultBranch

	ctxUser := common.CheckContextUser(ctx, ctx.FormInt64("org"))
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	ctx.Data["repo_template_name"] = ctx.Tr("repo.template_select")
	templateID := ctx.FormInt64("template_id")
	if templateID > 0 {
		templateRepo, err := repo_model.GetRepositoryByID(templateID)
		if err == nil && access_model.CheckRepoUnitUser(ctx, templateRepo, ctxUser, unit.TypeCode) {
			ctx.Data["repo_template"] = templateID
			ctx.Data["repo_template_name"] = templateRepo.Name
		}
	}

	ctx.Data["CanCreateRepo"] = ctx.Doer.CanCreateRepo()
	ctx.Data["MaxCreationLimit"] = ctx.Doer.MaxCreationLimit()

	ctx.HTML(http.StatusOK, tplCreate)
}

func handleCreateError(ctx *context.Context, owner *user_model.User, err error, name string, tpl base.TplName, form interface{}) {
	switch {
	case repo_model.IsErrReachLimitOfRepo(err):
		maxCreationLimit := owner.MaxCreationLimit()
		msg := ctx.TrN(maxCreationLimit, "repo.form.reach_limit_of_creation_1", "repo.form.reach_limit_of_creation_n", maxCreationLimit)
		ctx.RenderWithErr(msg, tpl, form)
	case repo_model.IsErrRepoAlreadyExist(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("form.repo_name_been_taken"), tpl, form)
	case repo_model.IsErrRepoFilesAlreadyExist(err):
		ctx.Data["Err_RepoName"] = true
		switch {
		case ctx.IsUserSiteAdmin() || (setting.Repository.AllowAdoptionOfUnadoptedRepositories && setting.Repository.AllowDeleteOfUnadoptedRepositories):
			ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist.adopt_or_delete"), tpl, form)
		case setting.Repository.AllowAdoptionOfUnadoptedRepositories:
			ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist.adopt"), tpl, form)
		case setting.Repository.AllowDeleteOfUnadoptedRepositories:
			ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist.delete"), tpl, form)
		default:
			ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist"), tpl, form)
		}
	case db.IsErrNameReserved(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("repo.form.name_reserved", err.(db.ErrNameReserved).Name), tpl, form)
	case db.IsErrNamePatternNotAllowed(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("repo.form.name_pattern_not_allowed", err.(db.ErrNamePatternNotAllowed).Pattern), tpl, form)
	default:
		ctx.ServerError(name, err)
	}
}

func getRepository(ctx *context.Context, repoID int64) *repo_model.Repository {
	repo, err := repo_model.GetRepositoryByID(repoID)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			ctx.NotFound("GetRepositoryByID", nil)
		} else {
			ctx.ServerError("GetRepositoryByID", err)
		}
		return nil
	}

	perm, err := access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return nil
	}

	if !perm.CanRead(unit.TypeCode) {
		log.Trace("Permission Denied: User %-v cannot read %-v of repo %-v\n"+
			"User in repo has Permissions: %-+v",
			ctx.Doer,
			unit.TypeCode,
			ctx.Repo,
			perm)
		ctx.NotFound("getRepository", nil)
		return nil
	}
	return repo
}

// CreatePost response for creating repository
func CreatePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateRepoForm)
	ctx.Data["Title"] = ctx.Tr("new_repo")

	ctx.Data["Gitignores"] = repo_module.Gitignores
	ctx.Data["LabelTemplates"] = repo_module.LabelTemplates
	ctx.Data["Licenses"] = repo_module.Licenses
	ctx.Data["Readmes"] = repo_module.Readmes

	ctx.Data["CanCreateRepo"] = ctx.Doer.CanCreateRepo()
	ctx.Data["MaxCreationLimit"] = ctx.Doer.MaxCreationLimit()

	ctxUser := common.CheckContextUser(ctx, form.UID)
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplCreate)
		return
	}

	var repo *repo_model.Repository
	var err error
	if form.RepoTemplate > 0 {
		opts := repo_module.GenerateRepoOptions{
			Name:        form.RepoName,
			Description: form.Description,
			Private:     form.Private,
			GitContent:  form.GitContent,
			Topics:      form.Topics,
			GitHooks:    form.GitHooks,
			Webhooks:    form.Webhooks,
			Avatar:      form.Avatar,
			IssueLabels: form.Labels,
		}

		if !opts.IsValid() {
			ctx.RenderWithErr(ctx.Tr("repo.template.one_item"), tplCreate, form)
			return
		}

		templateRepo := getRepository(ctx, form.RepoTemplate)
		if ctx.Written() {
			return
		}

		if !templateRepo.IsTemplate {
			ctx.RenderWithErr(ctx.Tr("repo.template.invalid"), tplCreate, form)
			return
		}

		repo, err = repo_service.GenerateRepository(ctx.Doer, ctxUser, templateRepo, opts)
		if err == nil {
			log.Trace("Repository generated [%d]: %s/%s", repo.ID, ctxUser.Name, repo.Name)
			ctx.Redirect(repo.Link())
			return
		}
	} else {
		repo, err = repo_service.CreateRepository(ctx.Doer, ctxUser, repo_module.CreateRepoOptions{
			Name:          form.RepoName,
			Description:   form.Description,
			Gitignores:    form.Gitignores,
			IssueLabels:   form.IssueLabels,
			License:       form.License,
			Readme:        form.Readme,
			IsPrivate:     form.Private || setting.Repository.ForcePrivate,
			DefaultBranch: form.DefaultBranch,
			AutoInit:      form.AutoInit,
			IsTemplate:    form.Template,
			TrustModel:    repo_model.ToTrustModel(form.TrustModel),
		})
		if err == nil {
			log.Trace("Repository created [%d]: %s/%s", repo.ID, ctxUser.Name, repo.Name)
			ctx.Redirect(repo.Link())
			return
		}
	}

	handleCreateError(ctx, ctxUser, err, "CreatePost", tplCreate, &form)
}

// Action response for actions to a repository
func Action(ctx *context.Context) {
	var err error
	switch ctx.Params(":action") {
	case "watch":
		err = repo_model.WatchRepo(ctx, ctx.Doer.ID, ctx.Repo.Repository.ID, true)
	case "unwatch":
		err = repo_model.WatchRepo(ctx, ctx.Doer.ID, ctx.Repo.Repository.ID, false)
	case "star":
		err = repo_model.StarRepo(ctx.Doer.ID, ctx.Repo.Repository.ID, true)
	case "unstar":
		err = repo_model.StarRepo(ctx.Doer.ID, ctx.Repo.Repository.ID, false)
	case "accept_transfer":
		err = acceptOrRejectRepoTransfer(ctx, true)
	case "reject_transfer":
		err = acceptOrRejectRepoTransfer(ctx, false)
	case "desc": // FIXME: this is not used
		if !ctx.Repo.IsOwner() {
			ctx.Error(http.StatusNotFound)
			return
		}

		ctx.Repo.Repository.Description = ctx.FormString("desc")
		ctx.Repo.Repository.Website = ctx.FormString("site")
		err = repo_service.UpdateRepository(ctx.Repo.Repository, false)
	}

	if err != nil {
		ctx.ServerError(fmt.Sprintf("Action (%s)", ctx.Params(":action")), err)
		return
	}

	ctx.RedirectToFirst(ctx.FormString("redirect_to"), ctx.Repo.RepoLink)
}

func acceptOrRejectRepoTransfer(ctx *context.Context, accept bool) error {
	repoTransfer, err := models.GetPendingRepositoryTransfer(ctx.Repo.Repository)
	if err != nil {
		return err
	}

	if err := repoTransfer.LoadAttributes(); err != nil {
		return err
	}

	if !repoTransfer.CanUserAcceptTransfer(ctx.Doer) {
		return errors.New("user does not have enough permissions")
	}

	if accept {
		if ctx.Repo.GitRepo != nil {
			ctx.Repo.GitRepo.Close()
			ctx.Repo.GitRepo = nil
		}

		if err := repo_service.TransferOwnership(repoTransfer.Doer, repoTransfer.Recipient, ctx.Repo.Repository, repoTransfer.Teams); err != nil {
			return err
		}
		ctx.Flash.Success(ctx.Tr("repo.settings.transfer.success"))
	} else {
		if err := models.CancelRepositoryTransfer(ctx.Repo.Repository); err != nil {
			return err
		}
		ctx.Flash.Success(ctx.Tr("repo.settings.transfer.rejected"))
	}

	ctx.Redirect(ctx.Repo.Repository.HTMLURL())
	return nil
}

// RedirectDownload return a file based on the following infos:
func RedirectDownload(ctx *context.Context) {
	var (
		vTag     = ctx.Params("vTag")
		fileName = ctx.Params("fileName")
	)
	tagNames := []string{vTag}
	curRepo := ctx.Repo.Repository
	releases, err := repo_model.GetReleasesByRepoIDAndNames(ctx, curRepo.ID, tagNames)
	if err != nil {
		if repo_model.IsErrAttachmentNotExist(err) {
			ctx.Error(http.StatusNotFound)
			return
		}
		ctx.ServerError("RedirectDownload", err)
		return
	}
	if len(releases) == 1 {
		release := releases[0]
		att, err := repo_model.GetAttachmentByReleaseIDFileName(ctx, release.ID, fileName)
		if err != nil {
			ctx.Error(http.StatusNotFound)
			return
		}
		if att != nil {
			ctx.Redirect(att.DownloadURL())
			return
		}
	}
	ctx.Error(http.StatusNotFound)
}

// RenderUserCards render a page show users according the input template
func RenderUserCards(ctx *context.Context, total int, getter func(opts db.ListOptions) ([]*user_model.User, error), tpl base.TplName) {
	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}
	pager := context.NewPagination(total, setting.ItemsPerPage, page, 5)
	ctx.Data["Page"] = pager

	items, err := getter(db.ListOptions{
		Page:     pager.Paginater.Current(),
		PageSize: setting.ItemsPerPage,
	})
	if err != nil {
		ctx.ServerError("getter", err)
		return
	}
	ctx.Data["Cards"] = items

	ctx.HTML(http.StatusOK, tpl)
}

// Watchers render repository's watch users
func Watchers(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.watchers")
	ctx.Data["CardsTitle"] = ctx.Tr("repo.watchers")
	ctx.Data["PageIsWatchers"] = true

	RenderUserCards(ctx, ctx.Repo.Repository.NumWatches, func(opts db.ListOptions) ([]*user_model.User, error) {
		return repo_model.GetRepoWatchers(ctx.Repo.Repository.ID, opts)
	}, tplWatchers)
}

// Stars render repository's starred users
func Stars(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.stargazers")
	ctx.Data["CardsTitle"] = ctx.Tr("repo.stargazers")
	ctx.Data["PageIsStargazers"] = true
	RenderUserCards(ctx, ctx.Repo.Repository.NumStars, func(opts db.ListOptions) ([]*user_model.User, error) {
		return repo_model.GetStargazers(ctx.Repo.Repository, opts)
	}, tplWatchers)
}
