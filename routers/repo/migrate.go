// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/migrations"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/task"
	"code.gitea.io/gitea/modules/util"
)

const (
	tplMigrate base.TplName = "repo/migrate/migrate"
)

// Migrate render migration of repository page
func Migrate(ctx *context.Context) {
	ctx.Data["Services"] = append([]structs.GitServiceType{structs.PlainGitService}, structs.SupportedFullGitService...)
	serviceType := ctx.QueryInt("service_type")
	if serviceType == 0 {
		ctx.Data["Org"] = ctx.Query("org")
		ctx.Data["Mirror"] = ctx.Query("mirror")

		ctx.HTML(200, tplMigrate)
		return
	}

	ctx.Data["Title"] = ctx.Tr("new_migrate")
	ctx.Data["private"] = getRepoPrivate(ctx)
	ctx.Data["IsForcedPrivate"] = setting.Repository.ForcePrivate
	ctx.Data["DisableMirrors"] = setting.Repository.DisableMirrors
	ctx.Data["mirror"] = ctx.Query("mirror") == "1"
	ctx.Data["wiki"] = ctx.Query("wiki") == "1"
	ctx.Data["milestones"] = ctx.Query("milestones") == "1"
	ctx.Data["labels"] = ctx.Query("labels") == "1"
	ctx.Data["issues"] = ctx.Query("issues") == "1"
	ctx.Data["pull_requests"] = ctx.Query("pull_requests") == "1"
	ctx.Data["releases"] = ctx.Query("releases") == "1"
	ctx.Data["LFSActive"] = setting.LFS.StartServer
	// Plain git should be first
	ctx.Data["service"] = structs.GitServiceType(serviceType)

	ctxUser := checkContextUser(ctx, ctx.QueryInt64("org"))
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	ctx.HTML(200, base.TplName("repo/migrate/"+structs.GitServiceType(serviceType).Name()))
}

func handleMigrateError(ctx *context.Context, owner *models.User, err error, name string, tpl base.TplName, form *auth.MigrateRepoForm) {
	switch {
	case migrations.IsRateLimitError(err):
		ctx.RenderWithErr(ctx.Tr("form.visit_rate_limit"), tpl, form)
	case migrations.IsTwoFactorAuthError(err):
		ctx.RenderWithErr(ctx.Tr("form.2fa_auth_required"), tpl, form)
	case models.IsErrReachLimitOfRepo(err):
		ctx.RenderWithErr(ctx.Tr("repo.form.reach_limit_of_creation", owner.MaxCreationLimit()), tpl, form)
	case models.IsErrRepoAlreadyExist(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("form.repo_name_been_taken"), tpl, form)
	case models.IsErrRepoFilesAlreadyExist(err):
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
	case models.IsErrNameReserved(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("repo.form.name_reserved", err.(models.ErrNameReserved).Name), tpl, form)
	case models.IsErrNamePatternNotAllowed(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("repo.form.name_pattern_not_allowed", err.(models.ErrNamePatternNotAllowed).Pattern), tpl, form)
	default:
		remoteAddr, _ := auth.ParseRemoteAddr(form.CloneAddr, form.AuthUsername, form.AuthPassword, owner)
		err = util.URLSanitizedError(err, remoteAddr)
		if strings.Contains(err.Error(), "Authentication failed") ||
			strings.Contains(err.Error(), "Bad credentials") ||
			strings.Contains(err.Error(), "could not read Username") {
			ctx.Data["Err_Auth"] = true
			ctx.RenderWithErr(ctx.Tr("form.auth_failed", err.Error()), tpl, form)
		} else if strings.Contains(err.Error(), "fatal:") {
			ctx.Data["Err_CloneAddr"] = true
			ctx.RenderWithErr(ctx.Tr("repo.migrate.failed", err.Error()), tpl, form)
		} else {
			ctx.ServerError(name, err)
		}
	}
}

// MigratePost response for migrating from external git repository
func MigratePost(ctx *context.Context, form auth.MigrateRepoForm) {
	ctx.Data["Title"] = ctx.Tr("new_migrate")
	// Plain git should be first
	ctx.Data["service"] = structs.GitServiceType(form.Service)
	ctx.Data["Services"] = append([]structs.GitServiceType{structs.PlainGitService}, structs.SupportedFullGitService...)

	tpl := base.TplName("repo/migrate/" + structs.GitServiceType(form.Service).Name())

	ctxUser := checkContextUser(ctx, form.UID)
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	if ctx.HasError() {
		ctx.HTML(200, tpl)
		return
	}

	remoteAddr, err := auth.ParseRemoteAddr(form.CloneAddr, form.AuthUsername, form.AuthPassword, ctx.User)
	if err != nil {
		if models.IsErrInvalidCloneAddr(err) {
			ctx.Data["Err_CloneAddr"] = true
			addrErr := err.(models.ErrInvalidCloneAddr)
			switch {
			case addrErr.IsURLError:
				ctx.RenderWithErr(ctx.Tr("form.url_error"), tpl, &form)
			case addrErr.IsPermissionDenied:
				ctx.RenderWithErr(ctx.Tr("repo.migrate.permission_denied"), tpl, &form)
			case addrErr.IsInvalidPath:
				ctx.RenderWithErr(ctx.Tr("repo.migrate.invalid_local_path"), tpl, &form)
			default:
				ctx.ServerError("Unknown error", err)
			}
		} else {
			ctx.ServerError("ParseRemoteAddr", err)
		}
		return
	}

	var opts = migrations.MigrateOptions{
		OriginalURL:    form.CloneAddr,
		GitServiceType: structs.GitServiceType(form.Service),
		CloneAddr:      remoteAddr,
		RepoName:       form.RepoName,
		Description:    form.Description,
		Private:        form.Private || setting.Repository.ForcePrivate,
		Mirror:         form.Mirror && !setting.Repository.DisableMirrors,
		AuthUsername:   form.AuthUsername,
		AuthPassword:   form.AuthPassword,
		AuthToken:      form.AuthToken,
		Wiki:           form.Wiki,
		Issues:         form.Issues,
		Milestones:     form.Milestones,
		Labels:         form.Labels,
		Comments:       form.Issues || form.PullRequests,
		PullRequests:   form.PullRequests,
		Releases:       form.Releases,
	}
	if opts.Mirror {
		opts.Issues = false
		opts.Milestones = false
		opts.Labels = false
		opts.Comments = false
		opts.PullRequests = false
		opts.Releases = false
	}

	err = models.CheckCreateRepository(ctx.User, ctxUser, opts.RepoName, false)
	if err != nil {
		handleMigrateError(ctx, ctxUser, err, "MigratePost", tpl, &form)
		return
	}

	err = task.MigrateRepository(ctx.User, ctxUser, opts)
	if err == nil {
		ctx.Redirect(setting.AppSubURL + "/" + ctxUser.Name + "/" + opts.RepoName)
		return
	}

	handleMigrateError(ctx, ctxUser, err, "MigratePost", tpl, &form)
}
