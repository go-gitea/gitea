// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"net/url"
	"strings"

	admin_model "code.gitea.io/gitea/models/admin"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/migrations"
	"code.gitea.io/gitea/services/task"
)

const (
	tplMigrate templates.TplName = "repo/migrate/migrate"
)

// Migrate render migration of repository page
func Migrate(ctx *context.Context) {
	if setting.Repository.DisableMigrations {
		ctx.HTTPError(http.StatusForbidden, "Migrate: the site administrator has disabled migrations")
		return
	}

	serviceType := structs.GitServiceType(ctx.FormInt("service_type"))

	setMigrationContextData(ctx, serviceType)

	if serviceType == 0 {
		ctx.Data["Org"] = ctx.FormString("org")
		ctx.Data["Mirror"] = ctx.FormString("mirror")

		ctx.HTML(http.StatusOK, tplMigrate)
		return
	}

	ctx.Data["private"] = getRepoPrivate(ctx)
	ctx.Data["mirror"] = ctx.FormString("mirror") == "1"
	ctx.Data["lfs"] = ctx.FormString("lfs") == "1"
	ctx.Data["wiki"] = ctx.FormString("wiki") == "1"
	ctx.Data["milestones"] = ctx.FormString("milestones") == "1"
	ctx.Data["labels"] = ctx.FormString("labels") == "1"
	ctx.Data["issues"] = ctx.FormString("issues") == "1"
	ctx.Data["pull_requests"] = ctx.FormString("pull_requests") == "1"
	ctx.Data["releases"] = ctx.FormString("releases") == "1"

	ctxUser := checkContextUser(ctx, ctx.FormInt64("org"))
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	ctx.HTML(http.StatusOK, templates.TplName("repo/migrate/"+serviceType.Name()))
}

func handleMigrateError(ctx *context.Context, owner *user_model.User, err error, name string, tpl templates.TplName, form *forms.MigrateRepoForm) {
	if setting.Repository.DisableMigrations {
		ctx.HTTPError(http.StatusForbidden, "MigrateError: the site administrator has disabled migrations")
		return
	}

	switch {
	case migrations.IsRateLimitError(err):
		ctx.RenderWithErr(ctx.Tr("form.visit_rate_limit"), tpl, form)
	case migrations.IsTwoFactorAuthError(err):
		ctx.RenderWithErr(ctx.Tr("form.2fa_auth_required"), tpl, form)
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
		err = util.SanitizeErrorCredentialURLs(err)
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

func handleMigrateRemoteAddrError(ctx *context.Context, err error, tpl templates.TplName, form *forms.MigrateRepoForm) {
	if git.IsErrInvalidCloneAddr(err) {
		addrErr := err.(*git.ErrInvalidCloneAddr)
		switch {
		case addrErr.IsProtocolInvalid:
			ctx.RenderWithErr(ctx.Tr("repo.mirror_address_protocol_invalid"), tpl, form)
		case addrErr.IsURLError:
			ctx.RenderWithErr(ctx.Tr("form.url_error", addrErr.Host), tpl, form)
		case addrErr.IsPermissionDenied:
			if addrErr.LocalPath {
				ctx.RenderWithErr(ctx.Tr("repo.migrate.permission_denied"), tpl, form)
			} else {
				ctx.RenderWithErr(ctx.Tr("repo.migrate.permission_denied_blocked"), tpl, form)
			}
		case addrErr.IsInvalidPath:
			ctx.RenderWithErr(ctx.Tr("repo.migrate.invalid_local_path"), tpl, form)
		default:
			log.Error("Error whilst updating url: %v", err)
			ctx.RenderWithErr(ctx.Tr("form.url_error", "unknown"), tpl, form)
		}
	} else {
		log.Error("Error whilst updating url: %v", err)
		ctx.RenderWithErr(ctx.Tr("form.url_error", "unknown"), tpl, form)
	}
}

// MigratePost response for migrating from external git repository
func MigratePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.MigrateRepoForm)
	if setting.Repository.DisableMigrations {
		ctx.HTTPError(http.StatusForbidden, "MigratePost: the site administrator has disabled migrations")
		return
	}

	if form.Mirror && setting.Mirror.DisableNewPull {
		ctx.HTTPError(http.StatusBadRequest, "MigratePost: the site administrator has disabled creation of new mirrors")
		return
	}

	setMigrationContextData(ctx, form.Service)

	ctxUser := checkContextUser(ctx, form.UID)
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	tpl := templates.TplName("repo/migrate/" + form.Service.Name())

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tpl)
		return
	}

	remoteAddr, err := git.ParseRemoteAddr(form.CloneAddr, form.AuthUsername, form.AuthPassword)
	if err == nil {
		err = migrations.IsMigrateURLAllowed(remoteAddr, ctx.Doer)
	}
	if err != nil {
		ctx.Data["Err_CloneAddr"] = true
		handleMigrateRemoteAddrError(ctx, err, tpl, form)
		return
	}

	form.LFS = form.LFS && setting.LFS.StartServer

	if form.LFS && len(form.LFSEndpoint) > 0 {
		ep := lfs.DetermineEndpoint("", form.LFSEndpoint)
		if ep == nil {
			ctx.Data["Err_LFSEndpoint"] = true
			ctx.RenderWithErr(ctx.Tr("repo.migrate.invalid_lfs_endpoint"), tpl, &form)
			return
		}
		err = migrations.IsMigrateURLAllowed(ep.String(), ctx.Doer)
		if err != nil {
			ctx.Data["Err_LFSEndpoint"] = true
			handleMigrateRemoteAddrError(ctx, err, tpl, form)
			return
		}
	}

	opts := migrations.MigrateOptions{
		OriginalURL:    form.CloneAddr,
		GitServiceType: form.Service,
		CloneAddr:      remoteAddr,
		RepoName:       form.RepoName,
		Description:    form.Description,
		Private:        form.Private || setting.Repository.ForcePrivate,
		Mirror:         form.Mirror,
		LFS:            form.LFS,
		LFSEndpoint:    form.LFSEndpoint,
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
	if form.Service == structs.CodeCommitService {
		opts.AWSAccessKeyID = form.AWSAccessKeyID
		opts.AWSSecretAccessKey = form.AWSSecretAccessKey
	}

	err = repo_model.CheckCreateRepository(ctx, ctx.Doer, ctxUser, opts.RepoName, false)
	if err != nil {
		handleMigrateError(ctx, ctxUser, err, "MigratePost", tpl, form)
		return
	}

	err = task.MigrateRepository(ctx, ctx.Doer, ctxUser, opts)
	if err == nil {
		ctx.Redirect(ctxUser.HomeLink() + "/" + url.PathEscape(opts.RepoName))
		return
	}

	handleMigrateError(ctx, ctxUser, err, "MigratePost", tpl, form)
}

func setMigrationContextData(ctx *context.Context, serviceType structs.GitServiceType) {
	ctx.Data["Title"] = ctx.Tr("new_migrate")

	ctx.Data["LFSActive"] = setting.LFS.StartServer
	ctx.Data["IsForcedPrivate"] = setting.Repository.ForcePrivate
	ctx.Data["DisableNewPullMirrors"] = setting.Mirror.DisableNewPull

	// Plain git should be first
	ctx.Data["Services"] = append([]structs.GitServiceType{structs.PlainGitService}, structs.SupportedFullGitService...)
	ctx.Data["service"] = serviceType
}

func MigrateRetryPost(ctx *context.Context) {
	if err := task.RetryMigrateTask(ctx, ctx.Repo.Repository.ID); err != nil {
		log.Error("Retry task failed: %v", err)
		ctx.ServerError("task.RetryMigrateTask", err)
		return
	}
	ctx.JSONOK()
}

func MigrateCancelPost(ctx *context.Context) {
	migratingTask, err := admin_model.GetMigratingTask(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		log.Error("GetMigratingTask: %v", err)
		ctx.Redirect(ctx.Repo.Repository.Link())
		return
	}
	if migratingTask.Status == structs.TaskStatusRunning {
		taskUpdate := &admin_model.Task{ID: migratingTask.ID, Status: structs.TaskStatusFailed, Message: "canceled"}
		if err = taskUpdate.UpdateCols(ctx, "status", "message"); err != nil {
			ctx.ServerError("task.UpdateCols", err)
			return
		}
	}
	ctx.Redirect(ctx.Repo.Repository.Link())
}

// MigrateStatus returns migrate task's status
func MigrateStatus(ctx *context.Context) {
	task, err := admin_model.GetMigratingTask(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		if admin_model.IsErrTaskDoesNotExist(err) {
			ctx.JSON(http.StatusNotFound, map[string]any{
				"err": "task does not exist or you do not have access to this task",
			})
			return
		}
		log.Error("GetMigratingTask: %v", err)
		ctx.JSON(http.StatusInternalServerError, map[string]any{
			"err": http.StatusText(http.StatusInternalServerError),
		})
		return
	}

	message := task.Message

	if task.Message != "" && task.Message[0] == '{' {
		// assume message is actually a translatable string
		var translatableMessage admin_model.TranslatableMessage
		if err := json.Unmarshal([]byte(message), &translatableMessage); err != nil {
			translatableMessage = admin_model.TranslatableMessage{
				Format: "migrate.migrating_failed.error",
				Args:   []any{task.Message},
			}
		}
		message = ctx.Locale.TrString(translatableMessage.Format, translatableMessage.Args...)
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"status":  task.Status,
		"message": message,
	})
}
