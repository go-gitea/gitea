// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/task"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
)

// Migrate migrate remote git repository to gitea
func Migrate(ctx *context.APIContext) {
	// swagger:operation POST /repos/migrate repository repoMigrate
	// ---
	// summary: Migrate a remote git repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/MigrateRepoOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Repository"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.MigrateRepoOptions)

	//get repoOwner
	if setting.Repository.DisableMigrations {
		ctx.Error(http.StatusForbidden, "MigrationsGlobalDisabled", fmt.Errorf("the site administrator has disabled migrations"))
		return
	}

	if form.Mirror && setting.Repository.DisableMirrors {
		ctx.Error(http.StatusForbidden, "MirrorsGlobalDisabled", fmt.Errorf("the site administrator has disabled mirrors"))
		return
	}

	form.LFS = form.LFS && setting.LFS.StartServer

	if form.LFS && len(form.LFSEndpoint) > 0 {
		ep := lfs.DetermineEndpoint("", form.LFSEndpoint)
		if ep == nil {
			ctx.Error(http.StatusInternalServerError, "", ctx.Tr("repo.migrate.invalid_lfs_endpoint"))
			return
		}
		err := migrations.IsMigrateURLAllowed(ep.String(), ctx.User)
		if err != nil {
			handleRemoteAddrError(ctx, err)
			return
		}
	}

	var (
		repoOwner *models.User
		err       error
	)
	if len(form.RepoOwner) != 0 {
		repoOwner, err = models.GetUserByName(form.RepoOwner)
	} else if form.RepoOwnerID != 0 {
		repoOwner, err = models.GetUserByID(form.RepoOwnerID)
	} else {
		repoOwner = ctx.User
	}
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetUser", err)
		}
		return
	}

	if ctx.HasError() {
		ctx.Error(http.StatusUnprocessableEntity, "", ctx.GetErrMsg())
		return
	}

	if !ctx.User.IsAdmin {
		if !repoOwner.IsOrganization() && ctx.User.ID != repoOwner.ID {
			ctx.Error(http.StatusForbidden, "", "Given user is not an organization.")
			return
		}

		if repoOwner.IsOrganization() {
			// Check ownership of organization.
			isOwner, err := repoOwner.IsOwnedBy(ctx.User.ID)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "IsOwnedBy", err)
				return
			} else if !isOwner {
				ctx.Error(http.StatusForbidden, "", "Given user is not owner of organization.")
				return
			}
		}
	}

	remoteAddr, err := forms.ParseRemoteAddr(form.CloneAddr, form.AuthUsername, form.AuthPassword)
	if err == nil {
		err = migrations.IsMigrateURLAllowed(remoteAddr, ctx.User)
	}
	if err != nil {
		handleRemoteAddrError(ctx, err)
		return
	}

	gitServiceType := convert.ToGitServiceType(form.Service)

	var opts = migrations.MigrateOptions{
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
		Issues:         form.Issues || form.PullRequests,
		Milestones:     form.Milestones,
		Labels:         form.Labels,
		Comments:       true,
		PullRequests:   form.PullRequests,
		Releases:       form.Releases,
		GitServiceType: gitServiceType,
		MirrorInterval: form.MirrorInterval,
	}
	if opts.Mirror {
		opts.Issues = false
		opts.Milestones = false
		opts.Labels = false
		opts.Comments = false
		opts.PullRequests = false
		opts.Releases = false
	}

	if err = models.CheckCreateRepository(ctx.User, repoOwner, opts.RepoName, false); err != nil {
		handleMigrateError(ctx, repoOwner, &opts, err)
		return
	}

	repo, err := task.MigrateRepository(ctx.User, repoOwner, opts)
	if err == nil {
		log.Trace("Repository migrated: %s/%s", repoOwner.Name, form.RepoName)
		ctx.JSON(http.StatusCreated, convert.ToRepo(repo, models.AccessModeAdmin))
		return
	}

	handleMigrateError(ctx, repoOwner, &opts, err)
}

func handleMigrateError(ctx *context.APIContext, repoOwner *models.User, migrationOpts *migrations.MigrateOptions, err error) {
	switch {
	case migrations.IsRateLimitError(err):
		ctx.Error(http.StatusUnprocessableEntity, "", "Remote visit addressed rate limitation.")
	case migrations.IsTwoFactorAuthError(err):
		ctx.Error(http.StatusUnprocessableEntity, "", "Remote visit required two factors authentication.")
	case models.IsErrReachLimitOfRepo(err):
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("You have already reached your limit of %d repositories.", repoOwner.MaxCreationLimit()))
	case models.IsErrRepoAlreadyExist(err):
		ctx.Error(http.StatusConflict, "", "The repository with the same name already exists.")
	case models.IsErrRepoFilesAlreadyExist(err):
		ctx.Error(http.StatusConflict, "", "Files already exist for this repository. Adopt them or delete them.")
	case models.IsErrNameReserved(err):
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("The username '%s' is reserved.", err.(models.ErrNameReserved).Name))
	case models.IsErrNameCharsNotAllowed(err):
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("The username '%s' contains invalid characters.", err.(models.ErrNameCharsNotAllowed).Name))
	case models.IsErrNamePatternNotAllowed(err):
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("The pattern '%s' is not allowed in a username.", err.(models.ErrNamePatternNotAllowed).Pattern))
	case models.IsErrInvalidCloneAddr(err):
		ctx.Error(http.StatusUnprocessableEntity, "", err)
	case base.IsErrNotSupported(err):
		ctx.Error(http.StatusUnprocessableEntity, "", err)
	default:
		remoteAddr, _ := forms.ParseRemoteAddr(migrationOpts.CloneAddr, migrationOpts.AuthUsername, migrationOpts.AuthPassword)
		err = util.NewStringURLSanitizedError(err, remoteAddr, true)
		if strings.Contains(err.Error(), "Authentication failed") ||
			strings.Contains(err.Error(), "Bad credentials") ||
			strings.Contains(err.Error(), "could not read Username") {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("Authentication failed: %v.", err))
		} else if strings.Contains(err.Error(), "fatal:") {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("Migration failed: %v.", err))
		} else {
			ctx.Error(http.StatusInternalServerError, "MigrateRepository", err)
		}
	}
}

func handleRemoteAddrError(ctx *context.APIContext, err error) {
	if models.IsErrInvalidCloneAddr(err) {
		addrErr := err.(*models.ErrInvalidCloneAddr)
		switch {
		case addrErr.IsURLError:
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		case addrErr.IsPermissionDenied:
			if addrErr.LocalPath {
				ctx.Error(http.StatusUnprocessableEntity, "", "You are not allowed to import local repositories.")
			} else if len(addrErr.PrivateNet) == 0 {
				ctx.Error(http.StatusUnprocessableEntity, "", "You are not allowed to import from blocked hosts.")
			} else {
				ctx.Error(http.StatusUnprocessableEntity, "", "You are not allowed to import from private IPs.")
			}
		case addrErr.IsInvalidPath:
			ctx.Error(http.StatusUnprocessableEntity, "", "Invalid local path, it does not exist or not a directory.")
		default:
			ctx.Error(http.StatusInternalServerError, "ParseRemoteAddr", "Unknown error type (ErrInvalidCloneAddr): "+err.Error())
		}
	} else {
		ctx.Error(http.StatusInternalServerError, "ParseRemoteAddr", err)
	}
}

// GetMigratingTask returns the migrating task by repo's id
func GetMigratingTask(ctx *context.APIContext) {
	// swagger:operation GET /repos/migrate/status task
	// ---
	// summary: Get the migration status of a repository by its id
	// produces:
	// - application/json
	// parameters:
	// - name: repo_id
	//   in: query
	//   description: repository id
	//   type: int64
	// responses:
	//   "200":
	//     "$ref": "#/responses/"
	//   "404":
	//	   "$ref": "#/response/"
	t, err := models.GetMigratingTask(ctx.FormInt64("repo_id"))

	if err != nil {
		ctx.JSON(http.StatusNotFound, err)
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"status":  t.Status,
		"err":     t.Message,
		"repo-id": t.RepoID,
		"start":   t.StartTime,
		"end":     t.EndTime,
	})
}
