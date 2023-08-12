// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/notification"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/migrations"
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
	//   "409":
	//     description: The repository with the same name already exists.
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.MigrateRepoOptions)

	// get repoOwner
	var (
		repoOwner *user_model.User
		err       error
	)
	if len(form.RepoOwner) != 0 {
		repoOwner, err = user_model.GetUserByName(ctx, form.RepoOwner)
	} else if form.RepoOwnerID != 0 {
		repoOwner, err = user_model.GetUserByID(ctx, form.RepoOwnerID)
	} else {
		repoOwner = ctx.Doer
	}
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetUser", err)
		}
		return
	}

	if ctx.HasAPIError() {
		ctx.Error(http.StatusUnprocessableEntity, "", ctx.GetErrMsg())
		return
	}

	if !ctx.Doer.IsAdmin {
		if !repoOwner.IsOrganization() && ctx.Doer.ID != repoOwner.ID {
			ctx.Error(http.StatusForbidden, "", "Given user is not an organization.")
			return
		}

		if repoOwner.IsOrganization() {
			// Check ownership of organization.
			isOwner, err := organization.OrgFromUser(repoOwner).IsOwnedBy(ctx.Doer.ID)
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
		err = migrations.IsMigrateURLAllowed(remoteAddr, ctx.Doer)
	}
	if err != nil {
		handleRemoteAddrError(ctx, err)
		return
	}

	gitServiceType := convert.ToGitServiceType(form.Service)

	if form.Mirror && setting.Mirror.DisableNewPull {
		ctx.Error(http.StatusForbidden, "MirrorsGlobalDisabled", fmt.Errorf("the site administrator has disabled the creation of new pull mirrors"))
		return
	}

	if setting.Repository.DisableMigrations {
		ctx.Error(http.StatusForbidden, "MigrationsGlobalDisabled", fmt.Errorf("the site administrator has disabled migrations"))
		return
	}

	form.LFS = form.LFS && setting.LFS.StartServer

	if form.LFS && len(form.LFSEndpoint) > 0 {
		ep := lfs.DetermineEndpoint("", form.LFSEndpoint)
		if ep == nil {
			ctx.Error(http.StatusInternalServerError, "", ctx.Tr("repo.migrate.invalid_lfs_endpoint"))
			return
		}
		err = migrations.IsMigrateURLAllowed(ep.String(), ctx.Doer)
		if err != nil {
			handleRemoteAddrError(ctx, err)
			return
		}
	}

	opts := migrations.MigrateOptions{
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

	repo, err := repo_module.CreateRepository(ctx.Doer, repoOwner, repo_module.CreateRepoOptions{
		Name:           opts.RepoName,
		Description:    opts.Description,
		OriginalURL:    form.CloneAddr,
		GitServiceType: gitServiceType,
		IsPrivate:      opts.Private,
		IsMirror:       opts.Mirror,
		Status:         repo_model.RepositoryBeingMigrated,
	})
	if err != nil {
		handleMigrateError(ctx, repoOwner, remoteAddr, err)
		return
	}

	opts.MigrateToRepoID = repo.ID

	defer func() {
		if e := recover(); e != nil {
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "Handler crashed with error: %v", log.Stack(2))

			err = errors.New(buf.String())
		}

		if err == nil {
			notification.NotifyMigrateRepository(ctx, ctx.Doer, repoOwner, repo)
			return
		}

		if repo != nil {
			if errDelete := models.DeleteRepository(ctx.Doer, repoOwner.ID, repo.ID); errDelete != nil {
				log.Error("DeleteRepository: %v", errDelete)
			}
		}
	}()

	if repo, err = migrations.MigrateRepository(graceful.GetManager().HammerContext(), ctx.Doer, repoOwner.Name, opts, nil); err != nil {
		handleMigrateError(ctx, repoOwner, remoteAddr, err)
		return
	}

	log.Trace("Repository migrated: %s/%s", repoOwner.Name, form.RepoName)
	ctx.JSON(http.StatusCreated, convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeAdmin}))
}

func handleMigrateError(ctx *context.APIContext, repoOwner *user_model.User, remoteAddr string, err error) {
	switch {
	case repo_model.IsErrRepoAlreadyExist(err):
		ctx.Error(http.StatusConflict, "", "The repository with the same name already exists.")
	case repo_model.IsErrRepoFilesAlreadyExist(err):
		ctx.Error(http.StatusConflict, "", "Files already exist for this repository. Adopt them or delete them.")
	case migrations.IsRateLimitError(err):
		ctx.Error(http.StatusUnprocessableEntity, "", "Remote visit addressed rate limitation.")
	case migrations.IsTwoFactorAuthError(err):
		ctx.Error(http.StatusUnprocessableEntity, "", "Remote visit required two factors authentication.")
	case repo_model.IsErrReachLimitOfRepo(err):
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("You have already reached your limit of %d repositories.", repoOwner.MaxCreationLimit()))
	case db.IsErrNameReserved(err):
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("The username '%s' is reserved.", err.(db.ErrNameReserved).Name))
	case db.IsErrNameCharsNotAllowed(err):
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("The username '%s' contains invalid characters.", err.(db.ErrNameCharsNotAllowed).Name))
	case db.IsErrNamePatternNotAllowed(err):
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("The pattern '%s' is not allowed in a username.", err.(db.ErrNamePatternNotAllowed).Pattern))
	case models.IsErrInvalidCloneAddr(err):
		ctx.Error(http.StatusUnprocessableEntity, "", err)
	case base.IsErrNotSupported(err):
		ctx.Error(http.StatusUnprocessableEntity, "", err)
	default:
		err = util.SanitizeErrorCredentialURLs(err)
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
			} else {
				ctx.Error(http.StatusUnprocessableEntity, "", "You can not import from disallowed hosts.")
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
