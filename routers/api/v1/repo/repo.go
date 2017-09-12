// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"strings"

	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/v1/convert"
)

// Search repositories via options
func Search(ctx *context.APIContext) {
	// swagger:route GET /repos/search repository repoSearch
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: SearchResults
	//       500: SearchError

	opts := &models.SearchRepoOptions{
		Keyword:  strings.Trim(ctx.Query("q"), " "),
		OwnerID:  ctx.QueryInt64("uid"),
		PageSize: convert.ToCorrectPageSize(ctx.QueryInt("limit")),
	}
	if ctx.User != nil && ctx.User.ID == opts.OwnerID {
		opts.Searcher = ctx.User
	}

	// Check visibility.
	if ctx.IsSigned && opts.OwnerID > 0 {
		if ctx.User.ID == opts.OwnerID {
			opts.Private = true
			opts.Collaborate = true
		} else {
			u, err := models.GetUserByID(opts.OwnerID)
			if err != nil {
				ctx.JSON(500, api.SearchError{
					OK:    false,
					Error: err.Error(),
				})
				return
			}
			if u.IsOrganization() && u.IsOwnedBy(ctx.User.ID) {
				opts.Private = true
			}

			if !u.IsOrganization() {
				opts.Collaborate = true
			}
		}
	}

	repos, count, err := models.SearchRepositoryByName(opts)
	if err != nil {
		ctx.JSON(500, api.SearchError{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	var userID int64
	if ctx.IsSigned {
		userID = ctx.User.ID
	}

	results := make([]*api.Repository, len(repos))
	for i, repo := range repos {
		if err = repo.GetOwner(); err != nil {
			ctx.JSON(500, api.SearchError{
				OK:    false,
				Error: err.Error(),
			})
			return
		}
		accessMode, err := models.AccessLevel(userID, repo)
		if err != nil {
			ctx.JSON(500, api.SearchError{
				OK:    false,
				Error: err.Error(),
			})
		}
		results[i] = repo.APIFormat(accessMode)
	}

	ctx.SetLinkHeader(int(count), setting.API.MaxResponseItems)
	ctx.Header().Set("X-Total-Count", fmt.Sprintf("%d", count))
	ctx.JSON(200, api.SearchResults{
		OK:   true,
		Data: results,
	})
}

// CreateUserRepo create a repository for a user
func CreateUserRepo(ctx *context.APIContext, owner *models.User, opt api.CreateRepoOption) {
	repo, err := models.CreateRepository(ctx.User, owner, models.CreateRepoOptions{
		Name:        opt.Name,
		Description: opt.Description,
		Gitignores:  opt.Gitignores,
		License:     opt.License,
		Readme:      opt.Readme,
		IsPrivate:   opt.Private,
		AutoInit:    opt.AutoInit,
	})
	if err != nil {
		if models.IsErrRepoAlreadyExist(err) ||
			models.IsErrNameReserved(err) ||
			models.IsErrNamePatternNotAllowed(err) {
			ctx.Error(422, "", err)
		} else {
			if repo != nil {
				if err = models.DeleteRepository(ctx.User, ctx.User.ID, repo.ID); err != nil {
					log.Error(4, "DeleteRepository: %v", err)
				}
			}
			ctx.Error(500, "CreateRepository", err)
		}
		return
	}

	ctx.JSON(201, repo.APIFormat(models.AccessModeOwner))
}

// Create one repository of mine
func Create(ctx *context.APIContext, opt api.CreateRepoOption) {
	// swagger:route POST /user/repos repository user createCurrentUserRepo
	//
	//     Consumes:
	//     - application/json
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       201: Repository
	//       403: forbidden
	//       422: validationError
	//       500: error

	// Shouldn't reach this condition, but just in case.
	if ctx.User.IsOrganization() {
		ctx.Error(422, "", "not allowed creating repository for organization")
		return
	}
	CreateUserRepo(ctx, ctx.User, opt)
}

// CreateOrgRepo create one repository of the organization
func CreateOrgRepo(ctx *context.APIContext, opt api.CreateRepoOption) {
	// swagger:route POST /org/{org}/repos organization createOrgRepo
	//
	//     Consumes:
	//     - application/json
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       201: Repository
	//       422: validationError
	//       403: forbidden
	//       500: error

	org, err := models.GetOrgByName(ctx.Params(":org"))
	if err != nil {
		if models.IsErrOrgNotExist(err) {
			ctx.Error(422, "", err)
		} else {
			ctx.Error(500, "GetOrgByName", err)
		}
		return
	}

	if !org.IsOwnedBy(ctx.User.ID) {
		ctx.Error(403, "", "Given user is not owner of organization.")
		return
	}
	CreateUserRepo(ctx, org, opt)
}

// Migrate migrate remote git repository to gitea
func Migrate(ctx *context.APIContext, form auth.MigrateRepoForm) {
	// swagger:route POST /repos/migrate repository repoMigrate
	//
	//     Consumes:
	//     - application/json
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       201: Repository
	//       422: validationError
	//       500: error

	ctxUser := ctx.User
	// Not equal means context user is an organization,
	// or is another user/organization if current user is admin.
	if form.UID != ctxUser.ID {
		org, err := models.GetUserByID(form.UID)
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.Error(422, "", err)
			} else {
				ctx.Error(500, "GetUserByID", err)
			}
			return
		}
		ctxUser = org
	}

	if ctx.HasError() {
		ctx.Error(422, "", ctx.GetErrMsg())
		return
	}

	if ctxUser.IsOrganization() && !ctx.User.IsAdmin {
		// Check ownership of organization.
		if !ctxUser.IsOwnedBy(ctx.User.ID) {
			ctx.Error(403, "", "Given user is not owner of organization.")
			return
		}
	}

	remoteAddr, err := form.ParseRemoteAddr(ctx.User)
	if err != nil {
		if models.IsErrInvalidCloneAddr(err) {
			addrErr := err.(models.ErrInvalidCloneAddr)
			switch {
			case addrErr.IsURLError:
				ctx.Error(422, "", err)
			case addrErr.IsPermissionDenied:
				ctx.Error(422, "", "You are not allowed to import local repositories.")
			case addrErr.IsInvalidPath:
				ctx.Error(422, "", "Invalid local path, it does not exist or not a directory.")
			default:
				ctx.Error(500, "ParseRemoteAddr", "Unknown error type (ErrInvalidCloneAddr): "+err.Error())
			}
		} else {
			ctx.Error(500, "ParseRemoteAddr", err)
		}
		return
	}

	repo, err := models.MigrateRepository(ctx.User, ctxUser, models.MigrateRepoOptions{
		Name:        form.RepoName,
		Description: form.Description,
		IsPrivate:   form.Private || setting.Repository.ForcePrivate,
		IsMirror:    form.Mirror,
		RemoteAddr:  remoteAddr,
	})
	if err != nil {
		if repo != nil {
			if errDelete := models.DeleteRepository(ctx.User, ctxUser.ID, repo.ID); errDelete != nil {
				log.Error(4, "DeleteRepository: %v", errDelete)
			}
		}
		ctx.Error(500, "MigrateRepository", models.HandleCloneUserCredentials(err.Error(), true))
		return
	}

	log.Trace("Repository migrated: %s/%s", ctxUser.Name, form.RepoName)
	ctx.JSON(201, repo.APIFormat(models.AccessModeAdmin))
}

// Get one repository
func Get(ctx *context.APIContext) {
	// swagger:route GET /repos/{username}/{reponame} repository repoGet
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: Repository
	//       500: error

	ctx.JSON(200, ctx.Repo.Repository.APIFormat(ctx.Repo.AccessMode))
}

// GetByID returns a single Repository
func GetByID(ctx *context.APIContext) {
	// swagger:route GET /repositories/{id} repository repoGetByID
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: Repository
	//       500: error

	repo, err := models.GetRepositoryByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrRepoNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetRepositoryByID", err)
		}
		return
	}

	access, err := models.AccessLevel(ctx.User.ID, repo)
	if err != nil {
		ctx.Error(500, "AccessLevel", err)
		return
	} else if access < models.AccessModeRead {
		ctx.Status(404)
		return
	}
	ctx.JSON(200, repo.APIFormat(access))
}

// Delete one repository
func Delete(ctx *context.APIContext) {
	// swagger:route DELETE /repos/{username}/{reponame} repository repoDelete
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       204: empty
	//       403: forbidden
	//       500: error

	if !ctx.Repo.IsAdmin() {
		ctx.Error(403, "", "Must have admin rights")
		return
	}
	owner := ctx.Repo.Owner
	repo := ctx.Repo.Repository

	if owner.IsOrganization() && !owner.IsOwnedBy(ctx.User.ID) {
		ctx.Error(403, "", "Given user is not owner of organization.")
		return
	}

	if err := models.DeleteRepository(ctx.User, owner.ID, repo.ID); err != nil {
		ctx.Error(500, "DeleteRepository", err)
		return
	}

	log.Trace("Repository deleted: %s/%s", owner.Name, repo.Name)
	ctx.Status(204)
}

// MirrorSync adds a mirrored repository to the sync queue
func MirrorSync(ctx *context.APIContext) {
	// swagger:route POST /repos/{username}/{reponame}/mirror-sync repository repoMirrorSync
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: empty
	//       403: forbidden

	repo := ctx.Repo.Repository

	if !ctx.Repo.IsWriter() {
		ctx.Error(403, "MirrorSync", "Must have write access")
	}

	go models.MirrorQueue.Add(repo.ID)
	ctx.Status(200)
}
