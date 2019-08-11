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
	"code.gitea.io/gitea/modules/migrations"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/v1/convert"

	api "code.gitea.io/gitea/modules/structs"
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
		UserIsAdmin: ctx.IsUserSiteAdmin(),
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
					log.Error("DeleteRepository: %v", err)
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
	//   "409":
	//     description: The repository with the same name already exists.
	//   "422":
	//     "$ref": "#/responses/validationError"
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

	var opts = migrations.MigrateOptions{
		RemoteURL:    remoteAddr,
		Name:         form.RepoName,
		Description:  form.Description,
		Private:      form.Private || setting.Repository.ForcePrivate,
		Mirror:       form.Mirror,
		AuthUsername: form.AuthUsername,
		AuthPassword: form.AuthPassword,
		Wiki:         form.Wiki,
		Issues:       form.Issues,
		Milestones:   form.Milestones,
		Labels:       form.Labels,
		Comments:     true,
		PullRequests: form.PullRequests,
		Releases:     form.Releases,
	}
	if opts.Mirror {
		opts.Issues = false
		opts.Milestones = false
		opts.Labels = false
		opts.Comments = false
		opts.PullRequests = false
		opts.Releases = false
	}

	repo, err := migrations.MigrateRepository(ctx.User, ctxUser.Name, opts)
	if err == nil {
		log.Trace("Repository migrated: %s/%s", ctxUser.Name, form.RepoName)
		ctx.JSON(201, repo.APIFormat(models.AccessModeAdmin))
		return
	}

	switch {
	case models.IsErrRepoAlreadyExist(err):
		ctx.Error(409, "", "The repository with the same name already exists.")
	case migrations.IsRateLimitError(err):
		ctx.Error(422, "", "Remote visit addressed rate limitation.")
	case migrations.IsTwoFactorAuthError(err):
		ctx.Error(422, "", "Remote visit required two factors authentication.")
	case models.IsErrReachLimitOfRepo(err):
		ctx.Error(422, "", fmt.Sprintf("You have already reached your limit of %d repositories.", ctxUser.MaxCreationLimit()))
	case models.IsErrNameReserved(err):
		ctx.Error(422, "", fmt.Sprintf("The username '%s' is reserved.", err.(models.ErrNameReserved).Name))
	case models.IsErrNamePatternNotAllowed(err):
		ctx.Error(422, "", fmt.Sprintf("The pattern '%s' is not allowed in a username.", err.(models.ErrNamePatternNotAllowed).Pattern))
	default:
		err = util.URLSanitizedError(err, remoteAddr)
		if strings.Contains(err.Error(), "Authentication failed") ||
			strings.Contains(err.Error(), "Bad credentials") ||
			strings.Contains(err.Error(), "could not read Username") {
			ctx.Error(422, "", fmt.Sprintf("Authentication failed: %v.", err))
		} else if strings.Contains(err.Error(), "fatal:") {
			ctx.Error(422, "", fmt.Sprintf("Migration failed: %v.", err))
		} else {
			ctx.Error(500, "MigrateRepository", err)
		}
	}
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
			ctx.NotFound()
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
		ctx.NotFound()
		return
	}
	ctx.JSON(200, repo.APIFormat(perm.AccessMode))
}

// Edit edit repository properties
func Edit(ctx *context.APIContext, opts api.EditRepoOption) {
	// swagger:operation PATCH /repos/{owner}/{repo} repository repoEdit
	// ---
	// summary: Edit a repository's properties. Only fields that are set will be changed.
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to edit
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to edit
	//   type: string
	//   required: true
	//   required: true
	// - name: body
	//   in: body
	//   description: "Properties of a repo that you can edit"
	//   schema:
	//     "$ref": "#/definitions/EditRepoOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Repository"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	if err := updateBasicProperties(ctx, opts); err != nil {
		return
	}

	if err := updateRepoUnits(ctx, opts); err != nil {
		return
	}

	if opts.Archived != nil {
		if err := updateRepoArchivedState(ctx, opts); err != nil {
			return
		}
	}

	ctx.JSON(http.StatusOK, ctx.Repo.Repository.APIFormat(ctx.Repo.AccessMode))
}

// updateBasicProperties updates the basic properties of a repo: Name, Description, Website and Visibility
func updateBasicProperties(ctx *context.APIContext, opts api.EditRepoOption) error {
	owner := ctx.Repo.Owner
	repo := ctx.Repo.Repository

	oldRepoName := repo.Name
	newRepoName := repo.Name
	if opts.Name != nil {
		newRepoName = *opts.Name
	}
	// Check if repository name has been changed and not just a case change
	if repo.LowerName != strings.ToLower(newRepoName) {
		if err := models.ChangeRepositoryName(ctx.Repo.Owner, repo.Name, newRepoName); err != nil {
			switch {
			case models.IsErrRepoAlreadyExist(err):
				ctx.Error(http.StatusUnprocessableEntity, fmt.Sprintf("repo name is already taken [name: %s]", newRepoName), err)
			case models.IsErrNameReserved(err):
				ctx.Error(http.StatusUnprocessableEntity, fmt.Sprintf("repo name is reserved [name: %s]", newRepoName), err)
			case models.IsErrNamePatternNotAllowed(err):
				ctx.Error(http.StatusUnprocessableEntity, fmt.Sprintf("repo name's pattern is not allowed [name: %s, pattern: %s]", newRepoName, err.(models.ErrNamePatternNotAllowed).Pattern), err)
			default:
				ctx.Error(http.StatusUnprocessableEntity, "ChangeRepositoryName", err)
			}
			return err
		}

		err := models.NewRepoRedirect(ctx.Repo.Owner.ID, repo.ID, repo.Name, newRepoName)
		if err != nil {
			ctx.Error(http.StatusUnprocessableEntity, "NewRepoRedirect", err)
			return err
		}

		if err := models.RenameRepoAction(ctx.User, oldRepoName, repo); err != nil {
			log.Error("RenameRepoAction: %v", err)
			ctx.Error(http.StatusInternalServerError, "RenameRepoActions", err)
			return err
		}

		log.Trace("Repository name changed: %s/%s -> %s", ctx.Repo.Owner.Name, repo.Name, newRepoName)
	}
	// Update the name in the repo object for the response
	repo.Name = newRepoName
	repo.LowerName = strings.ToLower(newRepoName)

	if opts.Description != nil {
		repo.Description = *opts.Description
	}

	if opts.Website != nil {
		repo.Website = *opts.Website
	}

	visibilityChanged := false
	if opts.Private != nil {
		// Visibility of forked repository is forced sync with base repository.
		if repo.IsFork {
			*opts.Private = repo.BaseRepo.IsPrivate
		}

		visibilityChanged = repo.IsPrivate != *opts.Private
		// when ForcePrivate enabled, you could change public repo to private, but only admin users can change private to public
		if visibilityChanged && setting.Repository.ForcePrivate && !*opts.Private && !ctx.User.IsAdmin {
			err := fmt.Errorf("cannot change private repository to public")
			ctx.Error(http.StatusUnprocessableEntity, "Force Private enabled", err)
			return err
		}

		repo.IsPrivate = *opts.Private
	}

	if err := models.UpdateRepository(repo, visibilityChanged); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateRepository", err)
		return err
	}

	log.Trace("Repository basic settings updated: %s/%s", owner.Name, repo.Name)
	return nil
}

// updateRepoUnits updates repo units: Issue settings, Wiki settings, PR settings
func updateRepoUnits(ctx *context.APIContext, opts api.EditRepoOption) error {
	owner := ctx.Repo.Owner
	repo := ctx.Repo.Repository

	var units []models.RepoUnit

	for _, tp := range models.MustRepoUnits {
		units = append(units, models.RepoUnit{
			RepoID: repo.ID,
			Type:   tp,
			Config: new(models.UnitConfig),
		})
	}

	if opts.HasIssues == nil {
		// If HasIssues setting not touched, rewrite existing repo unit
		if unit, err := repo.GetUnit(models.UnitTypeIssues); err == nil {
			units = append(units, *unit)
		} else if unit, err := repo.GetUnit(models.UnitTypeExternalTracker); err == nil {
			units = append(units, *unit)
		}
	} else if *opts.HasIssues {
		// We don't currently allow setting individual issue settings through the API,
		// only can enable/disable issues, so when enabling issues,
		// we either get the existing config which means it was already enabled,
		// or create a new config since it doesn't exist.
		unit, err := repo.GetUnit(models.UnitTypeIssues)
		var config *models.IssuesConfig
		if err != nil {
			// Unit type doesn't exist so we make a new config file with default values
			config = &models.IssuesConfig{
				EnableTimetracker:                true,
				AllowOnlyContributorsToTrackTime: true,
				EnableDependencies:               true,
			}
		} else {
			config = unit.IssuesConfig()
		}
		units = append(units, models.RepoUnit{
			RepoID: repo.ID,
			Type:   models.UnitTypeIssues,
			Config: config,
		})
	}

	if opts.HasWiki == nil {
		// If HasWiki setting not touched, rewrite existing repo unit
		if unit, err := repo.GetUnit(models.UnitTypeWiki); err == nil {
			units = append(units, *unit)
		} else if unit, err := repo.GetUnit(models.UnitTypeExternalWiki); err == nil {
			units = append(units, *unit)
		}
	} else if *opts.HasWiki {
		// We don't currently allow setting individual wiki settings through the API,
		// only can enable/disable the wiki, so when enabling the wiki,
		// we either get the existing config which means it was already enabled,
		// or create a new config since it doesn't exist.
		config := &models.UnitConfig{}
		units = append(units, models.RepoUnit{
			RepoID: repo.ID,
			Type:   models.UnitTypeWiki,
			Config: config,
		})
	}

	if opts.HasPullRequests == nil {
		// If HasPullRequest setting not touched, rewrite existing repo unit
		if unit, err := repo.GetUnit(models.UnitTypePullRequests); err == nil {
			units = append(units, *unit)
		}
	} else if *opts.HasPullRequests {
		// We do allow setting individual PR settings through the API, so
		// we get the config settings and then set them
		// if those settings were provided in the opts.
		unit, err := repo.GetUnit(models.UnitTypePullRequests)
		var config *models.PullRequestsConfig
		if err != nil {
			// Unit type doesn't exist so we make a new config file with default values
			config = &models.PullRequestsConfig{
				IgnoreWhitespaceConflicts: false,
				AllowMerge:                true,
				AllowRebase:               true,
				AllowRebaseMerge:          true,
				AllowSquash:               true,
			}
		} else {
			config = unit.PullRequestsConfig()
		}

		if opts.IgnoreWhitespaceConflicts != nil {
			config.IgnoreWhitespaceConflicts = *opts.IgnoreWhitespaceConflicts
		}
		if opts.AllowMerge != nil {
			config.AllowMerge = *opts.AllowMerge
		}
		if opts.AllowRebase != nil {
			config.AllowRebase = *opts.AllowRebase
		}
		if opts.AllowRebaseMerge != nil {
			config.AllowRebaseMerge = *opts.AllowRebaseMerge
		}
		if opts.AllowSquash != nil {
			config.AllowSquash = *opts.AllowSquash
		}

		units = append(units, models.RepoUnit{
			RepoID: repo.ID,
			Type:   models.UnitTypePullRequests,
			Config: config,
		})
	}

	if err := models.UpdateRepositoryUnits(repo, units); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateRepositoryUnits", err)
		return err
	}

	log.Trace("Repository advanced settings updated: %s/%s", owner.Name, repo.Name)
	return nil
}

// updateRepoArchivedState updates repo's archive state
func updateRepoArchivedState(ctx *context.APIContext, opts api.EditRepoOption) error {
	repo := ctx.Repo.Repository
	// archive / un-archive
	if opts.Archived != nil {
		if repo.IsMirror {
			err := fmt.Errorf("repo is a mirror, cannot archive/un-archive")
			ctx.Error(http.StatusUnprocessableEntity, err.Error(), err)
			return err
		}
		if *opts.Archived {
			if err := repo.SetArchiveRepoState(*opts.Archived); err != nil {
				log.Error("Tried to archive a repo: %s", err)
				ctx.Error(http.StatusInternalServerError, "ArchiveRepoState", err)
				return err
			}
			log.Trace("Repository was archived: %s/%s", ctx.Repo.Owner.Name, repo.Name)
		} else {
			if err := repo.SetArchiveRepoState(*opts.Archived); err != nil {
				log.Error("Tried to un-archive a repo: %s", err)
				ctx.Error(http.StatusInternalServerError, "ArchiveRepoState", err)
				return err
			}
			log.Trace("Repository was un-archived: %s/%s", ctx.Repo.Owner.Name, repo.Name)
		}
	}
	return nil
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
		log.Error("SearchTopics failed: %v", err)
		ctx.JSON(500, map[string]interface{}{
			"message": "Search topics failed.",
		})
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"topics": topics,
	})
}
