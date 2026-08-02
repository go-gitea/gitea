// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"net/url"
	"strings"

	admin_model "gitea.dev/models/admin"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
	"gitea.dev/services/migrations"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/services/task"
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

func handleMigrateError(ctx *context.Context, owner *user_model.User, err error, name string) {
	if setting.Repository.DisableMigrations {
		ctx.HTTPError(http.StatusForbidden, "the site administrator has disabled migrations")
		return
	}

	switch {
	case migrations.IsRateLimitError(err):
		ctx.JSONError(ctx.Tr("form.visit_rate_limit"))
	case migrations.IsTwoFactorAuthError(err):
		ctx.JSONError(ctx.Tr("form.2fa_auth_required"))
	case repo_model.IsErrReachLimitOfRepo(err):
		maxCreationLimit := owner.MaxCreationLimit()
		ctx.JSONError(ctx.TrN(maxCreationLimit, "repo.form.reach_limit_of_creation_1", "repo.form.reach_limit_of_creation_n", maxCreationLimit))
	case repo_model.IsErrRepoAlreadyExist(err):
		ctx.JSONError(ctx.Tr("form.repo_name_been_taken"))
	case repo_model.IsErrRepoFilesAlreadyExist(err):
		switch {
		case ctx.IsUserSiteAdmin() || (setting.Repository.AllowAdoptionOfUnadoptedRepositories && setting.Repository.AllowDeleteOfUnadoptedRepositories):
			ctx.JSONError(ctx.Tr("form.repository_files_already_exist.adopt_or_delete"))
		case setting.Repository.AllowAdoptionOfUnadoptedRepositories:
			ctx.JSONError(ctx.Tr("form.repository_files_already_exist.adopt"))
		case setting.Repository.AllowDeleteOfUnadoptedRepositories:
			ctx.JSONError(ctx.Tr("form.repository_files_already_exist.delete"))
		default:
			ctx.JSONError(ctx.Tr("form.repository_files_already_exist"))
		}
	case db.IsErrNameReserved(err):
		ctx.JSONError(ctx.Tr("repo.form.name_reserved", err.(db.ErrNameReserved).Name))
	case db.IsErrNamePatternNotAllowed(err):
		ctx.JSONError(ctx.Tr("repo.form.name_pattern_not_allowed", err.(db.ErrNamePatternNotAllowed).Pattern))
	default:
		err = util.SanitizeErrorCredentialURLs(err)
		if strings.Contains(err.Error(), "Authentication failed") ||
			strings.Contains(err.Error(), "Bad credentials") ||
			strings.Contains(err.Error(), "could not read Username") {
			ctx.JSONError(ctx.Tr("form.auth_failed", err.Error()))
		} else if strings.Contains(err.Error(), "fatal:") {
			ctx.JSONError(ctx.Tr("repo.migrate.failed", err.Error()))
		} else {
			ctx.ServerError(name, err)
		}
	}
}

func handleMigrateRemoteAddrError(ctx *context.Context, err error) {
	if git.IsErrInvalidCloneAddr(err) {
		addrErr := err.(*git.ErrInvalidCloneAddr)
		switch {
		case addrErr.IsProtocolInvalid:
			ctx.JSONError(ctx.Tr("repo.mirror_address_protocol_invalid"))
			return
		case addrErr.IsURLError:
			ctx.JSONError(ctx.Tr("form.url_error", addrErr.Host))
			return
		case addrErr.IsPermissionDenied:
			ctx.JSONError(ctx.Tr(util.Iif(addrErr.LocalPath, "repo.migrate.permission_denied", "repo.migrate.permission_denied_blocked")))
			return
		case addrErr.IsInvalidPath:
			ctx.JSONError(ctx.Tr("repo.migrate.invalid_local_path"))
			return
		}
	}
	log.Error("Error whilst updating url: %v", err)
	ctx.JSONError(ctx.Tr("form.url_error", "unknown"))
}

// MigratePost response for migrating from external git repository
func MigratePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.MigrateRepoForm)
	if setting.Repository.DisableMigrations {
		ctx.HTTPError(http.StatusForbidden, "the site administrator has disabled migrations")
		return
	}

	if form.Mirror && setting.Mirror.DisableNewPull {
		ctx.HTTPError(http.StatusBadRequest, "the site administrator has disabled creation of new mirrors")
		return
	}

	if ctx.HasError() { // form binding validation error
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	ctxUser := checkContextUser(ctx, form.UID)
	if ctx.Written() {
		return
	}

	remoteAddr, err := git.ParseRemoteAddr(form.CloneAddr, form.AuthUsername, form.AuthPassword)
	if err == nil {
		err = migrations.IsMigrateURLAllowed(remoteAddr, ctx.Doer)
	}
	if err != nil {
		handleMigrateRemoteAddrError(ctx, err)
		return
	}

	form.LFS = form.LFS && setting.LFS.StartServer

	if form.LFS && len(form.LFSEndpoint) > 0 {
		ep := lfs.DetermineEndpoint("", form.LFSEndpoint)
		if ep == nil {
			ctx.JSONError(ctx.Tr("repo.migrate.invalid_lfs_endpoint"))
			return
		}
		err = migrations.IsMigrateURLAllowed(ep.String(), ctx.Doer)
		if err != nil {
			handleMigrateRemoteAddrError(ctx, err)
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

	err = repo_service.CheckCreateRepository(ctx, ctx.Doer, ctxUser, opts.RepoName, false)
	if err != nil {
		handleMigrateError(ctx, ctxUser, err, "MigratePost")
		return
	}

	err = task.MigrateRepository(ctx, ctx.Doer, ctxUser, opts)
	if err == nil {
		ctx.JSONRedirect(ctxUser.HomeLink() + "/" + url.PathEscape(opts.RepoName))
		return
	}

	handleMigrateError(ctx, ctxUser, err, "MigratePost")
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
