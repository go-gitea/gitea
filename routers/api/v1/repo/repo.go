// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/v1/convert"

	api "code.gitea.io/sdk/gitea"
)

var searchOrderByMap = map[string]map[string]models.SearchOrderBy{
	"asc": {
		"alpha":   models.SearchOrderByAlphabetically,
		"created": models.SearchOrderByOldest,
		"updated": models.SearchOrderByLeastUpdated,
		"size":    models.SearchOrderBySize,
		"id":      models.SearchOrderByID,
	},
	"desc": {
		"alpha":   models.SearchOrderByAlphabeticallyReverse,
		"created": models.SearchOrderByNewest,
		"updated": models.SearchOrderByRecentUpdated,
		"size":    models.SearchOrderBySizeReverse,
		"id":      models.SearchOrderByIDReverse,
	},
}

// Search repositories via options
func Search(ctx *context.APIContext) {
	// swagger:operation GET /repos/search repository repoSearch
	// ---
	// summary: Search for repositories
	// produces:
	// - application/json
	// parameters:
	// - name: q
	//   in: query
	//   description: keyword
	//   type: string
	// - name: uid
	//   in: query
	//   description: search only for repos that the user with the given id owns or contributes to
	//   type: integer
	//   format: int64
	// - name: starredBy
	//   in: query
	//   description: search only for repos that the user with the given id has starred
	//   type: integer
	//   format: int64
	// - name: private
	//   in: query
	//   description: include private repositories this user has access to (defaults to true)
	//   type: boolean
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results, maximum page size is 50
	//   type: integer
	// - name: mode
	//   in: query
	//   description: type of repository to search for. Supported values are
	//                "fork", "source", "mirror" and "collaborative"
	//   type: string
	// - name: exclusive
	//   in: query
	//   description: if `uid` is given, search only for repos that the user owns
	//   type: boolean
	// - name: sort
	//   in: query
	//   description: sort repos by attribute. Supported values are
	//                "alpha", "created", "updated", "size", and "id".
	//                Default is "alpha"
	//   type: string
	// - name: order
	//   in: query
	//   description: sort order, either "asc" (ascending) or "desc" (descending).
	//                Default is "asc", ignored if "sort" is not specified.
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/SearchResults"
	//   "422":
	//     "$ref": "#/responses/validationError"
	opts := &models.SearchRepoOptions{
		Keyword:     strings.Trim(ctx.Query("q"), " "),
		OwnerID:     ctx.QueryInt64("uid"),
		Page:        ctx.QueryInt("page"),
		PageSize:    convert.ToCorrectPageSize(ctx.QueryInt("limit")),
		TopicOnly:   ctx.QueryBool("topic"),
		Collaborate: util.OptionalBoolNone,
		Private:     ctx.IsSigned && (ctx.Query("private") == "" || ctx.QueryBool("private")),
		UserIsAdmin: ctx.IsSigned && ctx.User.IsAdmin,
		UserID:      ctx.Data["SignedUserID"].(int64),
		StarredByID: ctx.QueryInt64("starredBy"),
	}

	if ctx.QueryBool("exclusive") {
		opts.Collaborate = util.OptionalBoolFalse
	}

	var mode = ctx.Query("mode")
	switch mode {
	case "source":
		opts.Fork = util.OptionalBoolFalse
		opts.Mirror = util.OptionalBoolFalse
	case "fork":
		opts.Fork = util.OptionalBoolTrue
	case "mirror":
		opts.Mirror = util.OptionalBoolTrue
	case "collaborative":
		opts.Mirror = util.OptionalBoolFalse
		opts.Collaborate = util.OptionalBoolTrue
	case "":
	default:
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("Invalid search mode: \"%s\"", mode))
		return
	}

	var sortMode = ctx.Query("sort")
	if len(sortMode) > 0 {
		var sortOrder = ctx.Query("order")
		if len(sortOrder) == 0 {
			sortOrder = "asc"
		}
		if searchModeMap, ok := searchOrderByMap[sortOrder]; ok {
			if orderBy, ok := searchModeMap[sortMode]; ok {
				opts.OrderBy = orderBy
			} else {
				ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("Invalid sort mode: \"%s\"", sortMode))
				return
			}
		} else {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("Invalid sort order: \"%s\"", sortOrder))
			return
		}
	}

	var err error
	repos, count, err := models.SearchRepositoryByName(opts)
	if err != nil {
		ctx.JSON(500, api.SearchError{
			OK:    false,
			Error: err.Error(),
		})
		return
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
		accessMode, err := models.AccessLevel(ctx.User, repo)
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
	if opt.AutoInit && opt.Readme == "" {
		opt.Readme = "Default"
	}
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
		if models.IsErrRepoAlreadyExist(err) {
			ctx.Error(409, "", "The repository with the same name already exists.")
		} else if models.IsErrNameReserved(err) ||
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
	// swagger:operation POST /user/repos repository user createCurrentUserRepo
	// ---
	// summary: Create a repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateRepoOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Repository"
	if ctx.User.IsOrganization() {
		// Shouldn't reach this condition, but just in case.
		ctx.Error(422, "", "not allowed creating repository for organization")
		return
	}
	CreateUserRepo(ctx, ctx.User, opt)
}

// CreateOrgRepo create one repository of the organization
func CreateOrgRepo(ctx *context.APIContext, opt api.CreateRepoOption) {
	// swagger:operation POST /org/{org}/repos organization createOrgRepo
	// ---
	// summary: Create a repository in an organization
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of organization
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateRepoOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Repository"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	org, err := models.GetOrgByName(ctx.Params(":org"))
	if err != nil {
		if models.IsErrOrgNotExist(err) {
			ctx.Error(422, "", err)
		} else {
			ctx.Error(500, "GetOrgByName", err)
		}
		return
	}

	if !models.HasOrgVisible(org, ctx.User) {
		ctx.NotFound("HasOrgVisible", nil)
		return
	}

	if !ctx.User.IsAdmin {
		isOwner, err := org.IsOwnedBy(ctx.User.ID)
		if err != nil {
			ctx.ServerError("IsOwnedBy", err)
			return
		} else if !isOwner {
			ctx.Error(403, "", "Given user is not owner of organization.")
			return
		}
	}
	CreateUserRepo(ctx, org, opt)
}

// Migrate migrate remote git repository to gitea
func Migrate(ctx *context.APIContext, form auth.MigrateRepoForm) {
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
	//     "$ref": "#/definitions/MigrateRepoForm"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Repository"
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

	if !ctx.User.IsAdmin {
		if !ctxUser.IsOrganization() && ctx.User.ID != ctxUser.ID {
			ctx.Error(403, "", "Given user is not an organization.")
			return
		}

		if ctxUser.IsOrganization() {
			// Check ownership of organization.
			isOwner, err := ctxUser.IsOwnedBy(ctx.User.ID)
			if err != nil {
				ctx.Error(500, "IsOwnedBy", err)
				return
			} else if !isOwner {
				ctx.Error(403, "", "Given user is not owner of organization.")
				return
			}
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
		if models.IsErrRepoAlreadyExist(err) {
			ctx.Error(409, "", "The repository with the same name already exists.")
			return
		}

		err = util.URLSanitizedError(err, remoteAddr)
		if repo != nil {
			if errDelete := models.DeleteRepository(ctx.User, ctxUser.ID, repo.ID); errDelete != nil {
				log.Error(4, "DeleteRepository: %v", errDelete)
			}
		}
		ctx.Error(500, "MigrateRepository", err)
		return
	}

	log.Trace("Repository migrated: %s/%s", ctxUser.Name, form.RepoName)
	ctx.JSON(201, repo.APIFormat(models.AccessModeAdmin))
}

// Get one repository
func Get(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo} repository repoGet
	// ---
	// summary: Get a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Repository"
	ctx.JSON(200, ctx.Repo.Repository.APIFormat(ctx.Repo.AccessMode))
}

// GetByID returns a single Repository
func GetByID(ctx *context.APIContext) {
	// swagger:operation GET /repositories/{id} repository repoGetByID
	// ---
	// summary: Get a repository by id
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the repo to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Repository"
	repo, err := models.GetRepositoryByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrRepoNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetRepositoryByID", err)
		}
		return
	}

	perm, err := models.GetUserRepoPermission(repo, ctx.User)
	if err != nil {
		ctx.Error(500, "AccessLevel", err)
		return
	} else if !perm.HasAccess() {
		ctx.Status(404)
		return
	}
	ctx.JSON(200, repo.APIFormat(perm.AccessMode))
}

// Delete one repository
func Delete(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo} repository repoDelete
	// ---
	// summary: Delete a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to delete
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to delete
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	owner := ctx.Repo.Owner
	repo := ctx.Repo.Repository

	if owner.IsOrganization() && !ctx.User.IsAdmin {
		isOwner, err := owner.IsOwnedBy(ctx.User.ID)
		if err != nil {
			ctx.Error(500, "IsOwnedBy", err)
			return
		} else if !isOwner {
			ctx.Error(403, "", "Given user is not owner of organization.")
			return
		}
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
	// swagger:operation POST /repos/{owner}/{repo}/mirror-sync repository repoMirrorSync
	// ---
	// summary: Sync a mirrored repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to sync
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to sync
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/empty"
	repo := ctx.Repo.Repository

	if !ctx.Repo.CanWrite(models.UnitTypeCode) {
		ctx.Error(403, "MirrorSync", "Must have write access")
	}

	go models.MirrorQueue.Add(repo.ID)
	ctx.Status(200)
}

// TopicSearch search for creating topic
func TopicSearch(ctx *context.Context) {
	// swagger:operation GET /topics/search repository topicSearch
	// ---
	// summary: search topics via keyword
	// produces:
	//   - application/json
	// parameters:
	//   - name: q
	//     in: query
	//     description: keywords to search
	//     required: true
	//     type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/Repository"
	if ctx.User == nil {
		ctx.JSON(403, map[string]interface{}{
			"message": "Only owners could change the topics.",
		})
		return
	}

	kw := ctx.Query("q")

	topics, err := models.FindTopics(&models.FindTopicOptions{
		Keyword: kw,
		Limit:   10,
	})
	if err != nil {
		log.Error(2, "SearchTopics failed: %v", err)
		ctx.JSON(500, map[string]interface{}{
			"message": "Search topics failed.",
		})
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"topics": topics,
	})
}
