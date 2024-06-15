// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/label"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/validation"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	actions_service "code.gitea.io/gitea/services/actions"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/issue"
	repo_service "code.gitea.io/gitea/services/repository"
)

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
	// - name: topic
	//   in: query
	//   description: Limit search to repositories with keyword as topic
	//   type: boolean
	// - name: includeDesc
	//   in: query
	//   description: include search of keyword within repository description
	//   type: boolean
	// - name: uid
	//   in: query
	//   description: search only for repos that the user with the given id owns or contributes to
	//   type: integer
	//   format: int64
	// - name: priority_owner_id
	//   in: query
	//   description: repo owner to prioritize in the results
	//   type: integer
	//   format: int64
	// - name: team_id
	//   in: query
	//   description: search only for repos that belong to the given team id
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
	// - name: is_private
	//   in: query
	//   description: show only pubic, private or all repositories (defaults to all)
	//   type: boolean
	// - name: template
	//   in: query
	//   description: include template repositories this user has access to (defaults to true)
	//   type: boolean
	// - name: archived
	//   in: query
	//   description: show only archived, non-archived or all repositories (defaults to all)
	//   type: boolean
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
	//                "alpha", "created", "updated", "size", "git_size", "lfs_size", "stars", "forks" and "id".
	//                Default is "alpha"
	//   type: string
	// - name: order
	//   in: query
	//   description: sort order, either "asc" (ascending) or "desc" (descending).
	//                Default is "asc", ignored if "sort" is not specified.
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/SearchResults"
	//   "422":
	//     "$ref": "#/responses/validationError"

	opts := &repo_model.SearchRepoOptions{
		ListOptions:        utils.GetListOptions(ctx),
		Actor:              ctx.Doer,
		Keyword:            ctx.FormTrim("q"),
		OwnerID:            ctx.FormInt64("uid"),
		PriorityOwnerID:    ctx.FormInt64("priority_owner_id"),
		TeamID:             ctx.FormInt64("team_id"),
		TopicOnly:          ctx.FormBool("topic"),
		Collaborate:        optional.None[bool](),
		Private:            ctx.IsSigned && (ctx.FormString("private") == "" || ctx.FormBool("private")),
		Template:           optional.None[bool](),
		StarredByID:        ctx.FormInt64("starredBy"),
		IncludeDescription: ctx.FormBool("includeDesc"),
	}

	if ctx.FormString("template") != "" {
		opts.Template = optional.Some(ctx.FormBool("template"))
	}

	if ctx.FormBool("exclusive") {
		opts.Collaborate = optional.Some(false)
	}

	mode := ctx.FormString("mode")
	switch mode {
	case "source":
		opts.Fork = optional.Some(false)
		opts.Mirror = optional.Some(false)
	case "fork":
		opts.Fork = optional.Some(true)
	case "mirror":
		opts.Mirror = optional.Some(true)
	case "collaborative":
		opts.Mirror = optional.Some(false)
		opts.Collaborate = optional.Some(true)
	case "":
	default:
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("Invalid search mode: \"%s\"", mode))
		return
	}

	if ctx.FormString("archived") != "" {
		opts.Archived = optional.Some(ctx.FormBool("archived"))
	}

	if ctx.FormString("is_private") != "" {
		opts.IsPrivate = optional.Some(ctx.FormBool("is_private"))
	}

	sortMode := ctx.FormString("sort")
	if len(sortMode) > 0 {
		sortOrder := ctx.FormString("order")
		if len(sortOrder) == 0 {
			sortOrder = "asc"
		}
		if searchModeMap, ok := repo_model.OrderByMap[sortOrder]; ok {
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
	repos, count, err := repo_model.SearchRepository(ctx, opts)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, api.SearchError{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	results := make([]*api.Repository, len(repos))
	for i, repo := range repos {
		if err = repo.LoadOwner(ctx); err != nil {
			ctx.JSON(http.StatusInternalServerError, api.SearchError{
				OK:    false,
				Error: err.Error(),
			})
			return
		}
		permission, err := access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, api.SearchError{
				OK:    false,
				Error: err.Error(),
			})
		}
		results[i] = convert.ToRepo(ctx, repo, permission)
	}
	ctx.SetLinkHeader(int(count), opts.PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, api.SearchResults{
		OK:   true,
		Data: results,
	})
}

// CreateUserRepo create a repository for a user
func CreateUserRepo(ctx *context.APIContext, owner *user_model.User, opt api.CreateRepoOption) {
	if opt.AutoInit && opt.Readme == "" {
		opt.Readme = "Default"
	}

	// If the readme template does not exist, a 400 will be returned.
	if opt.AutoInit && len(opt.Readme) > 0 && !slices.Contains(repo_module.Readmes, opt.Readme) {
		ctx.Error(http.StatusBadRequest, "", fmt.Errorf("readme template does not exist, available templates: %v", repo_module.Readmes))
		return
	}

	repo, err := repo_service.CreateRepository(ctx, ctx.Doer, owner, repo_service.CreateRepoOptions{
		Name:             opt.Name,
		Description:      opt.Description,
		IssueLabels:      opt.IssueLabels,
		Gitignores:       opt.Gitignores,
		License:          opt.License,
		Readme:           opt.Readme,
		IsPrivate:        opt.Private || setting.Repository.ForcePrivate,
		AutoInit:         opt.AutoInit,
		DefaultBranch:    opt.DefaultBranch,
		TrustModel:       repo_model.ToTrustModel(opt.TrustModel),
		IsTemplate:       opt.Template,
		ObjectFormatName: opt.ObjectFormatName,
	})
	if err != nil {
		if repo_model.IsErrRepoAlreadyExist(err) {
			ctx.Error(http.StatusConflict, "", "The repository with the same name already exists.")
		} else if db.IsErrNameReserved(err) ||
			db.IsErrNamePatternNotAllowed(err) ||
			label.IsErrTemplateLoad(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "CreateRepository", err)
		}
		return
	}

	// reload repo from db to get a real state after creation
	repo, err = repo_model.GetRepositoryByID(ctx, repo.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetRepositoryByID", err)
	}

	ctx.JSON(http.StatusCreated, convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeOwner}))
}

// Create one repository of mine
func Create(ctx *context.APIContext) {
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
	//   "400":
	//     "$ref": "#/responses/error"
	//   "409":
	//     description: The repository with the same name already exists.
	//   "422":
	//     "$ref": "#/responses/validationError"
	opt := web.GetForm(ctx).(*api.CreateRepoOption)
	if ctx.Doer.IsOrganization() {
		// Shouldn't reach this condition, but just in case.
		ctx.Error(http.StatusUnprocessableEntity, "", "not allowed creating repository for organization")
		return
	}
	CreateUserRepo(ctx, ctx.Doer, *opt)
}

// Generate Create a repository using a template
func Generate(ctx *context.APIContext) {
	// swagger:operation POST /repos/{template_owner}/{template_repo}/generate repository generateRepo
	// ---
	// summary: Create a repository using a template
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: template_owner
	//   in: path
	//   description: name of the template repository owner
	//   type: string
	//   required: true
	// - name: template_repo
	//   in: path
	//   description: name of the template repository
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/GenerateRepoOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Repository"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     description: The repository with the same name already exists.
	//   "422":
	//     "$ref": "#/responses/validationError"
	form := web.GetForm(ctx).(*api.GenerateRepoOption)

	if !ctx.Repo.Repository.IsTemplate {
		ctx.Error(http.StatusUnprocessableEntity, "", "this is not a template repo")
		return
	}

	if ctx.Doer.IsOrganization() {
		ctx.Error(http.StatusUnprocessableEntity, "", "not allowed creating repository for organization")
		return
	}

	opts := repo_service.GenerateRepoOptions{
		Name:            form.Name,
		DefaultBranch:   form.DefaultBranch,
		Description:     form.Description,
		Private:         form.Private || setting.Repository.ForcePrivate,
		GitContent:      form.GitContent,
		Topics:          form.Topics,
		GitHooks:        form.GitHooks,
		Webhooks:        form.Webhooks,
		Avatar:          form.Avatar,
		IssueLabels:     form.Labels,
		ProtectedBranch: form.ProtectedBranch,
	}

	if !opts.IsValid() {
		ctx.Error(http.StatusUnprocessableEntity, "", "must select at least one template item")
		return
	}

	ctxUser := ctx.Doer
	var err error
	if form.Owner != ctxUser.Name {
		ctxUser, err = user_model.GetUserByName(ctx, form.Owner)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.JSON(http.StatusNotFound, map[string]any{
					"error": "request owner `" + form.Owner + "` does not exist",
				})
				return
			}

			ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
			return
		}

		if !ctx.Doer.IsAdmin && !ctxUser.IsOrganization() {
			ctx.Error(http.StatusForbidden, "", "Only admin can generate repository for other user.")
			return
		}

		if !ctx.Doer.IsAdmin {
			canCreate, err := organization.OrgFromUser(ctxUser).CanCreateOrgRepo(ctx, ctx.Doer.ID)
			if err != nil {
				ctx.ServerError("CanCreateOrgRepo", err)
				return
			} else if !canCreate {
				ctx.Error(http.StatusForbidden, "", "Given user is not allowed to create repository in organization.")
				return
			}
		}
	}

	repo, err := repo_service.GenerateRepository(ctx, ctx.Doer, ctxUser, ctx.Repo.Repository, opts)
	if err != nil {
		if repo_model.IsErrRepoAlreadyExist(err) {
			ctx.Error(http.StatusConflict, "", "The repository with the same name already exists.")
		} else if db.IsErrNameReserved(err) ||
			db.IsErrNamePatternNotAllowed(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "CreateRepository", err)
		}
		return
	}
	log.Trace("Repository generated [%d]: %s/%s", repo.ID, ctxUser.Name, repo.Name)

	ctx.JSON(http.StatusCreated, convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeOwner}))
}

// CreateOrgRepoDeprecated create one repository of the organization
func CreateOrgRepoDeprecated(ctx *context.APIContext) {
	// swagger:operation POST /org/{org}/repos organization createOrgRepoDeprecated
	// ---
	// summary: Create a repository in an organization
	// deprecated: true
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	CreateOrgRepo(ctx)
}

// CreateOrgRepo create one repository of the organization
func CreateOrgRepo(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/repos organization createOrgRepo
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
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	opt := web.GetForm(ctx).(*api.CreateRepoOption)
	org, err := organization.GetOrgByName(ctx, ctx.Params(":org"))
	if err != nil {
		if organization.IsErrOrgNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetOrgByName", err)
		}
		return
	}

	if !organization.HasOrgOrUserVisible(ctx, org.AsUser(), ctx.Doer) {
		ctx.NotFound("HasOrgOrUserVisible", nil)
		return
	}

	if !ctx.Doer.IsAdmin {
		canCreate, err := org.CanCreateOrgRepo(ctx, ctx.Doer.ID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "CanCreateOrgRepo", err)
			return
		} else if !canCreate {
			ctx.Error(http.StatusForbidden, "", "Given user is not allowed to create repository in organization.")
			return
		}
	}
	CreateUserRepo(ctx, org.AsUser(), *opt)
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	if err := ctx.Repo.Repository.LoadAttributes(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "Repository.LoadAttributes", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToRepo(ctx, ctx.Repo.Repository, ctx.Repo.Permission))
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	repo, err := repo_model.GetRepositoryByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetRepositoryByID", err)
		}
		return
	}

	permission, err := access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
		return
	} else if !permission.HasAnyUnitAccess() {
		ctx.NotFound()
		return
	}
	ctx.JSON(http.StatusOK, convert.ToRepo(ctx, repo, permission))
}

// Edit edit repository properties
func Edit(ctx *context.APIContext) {
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
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	opts := *web.GetForm(ctx).(*api.EditRepoOption)

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

	if opts.MirrorInterval != nil || opts.EnablePrune != nil {
		if err := updateMirror(ctx, opts); err != nil {
			return
		}
	}

	repo, err := repo_model.GetRepositoryByID(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToRepo(ctx, repo, ctx.Repo.Permission))
}

// updateBasicProperties updates the basic properties of a repo: Name, Description, Website and Visibility
func updateBasicProperties(ctx *context.APIContext, opts api.EditRepoOption) error {
	owner := ctx.Repo.Owner
	repo := ctx.Repo.Repository
	newRepoName := repo.Name
	if opts.Name != nil {
		newRepoName = *opts.Name
	}
	// Check if repository name has been changed and not just a case change
	if repo.LowerName != strings.ToLower(newRepoName) {
		if err := repo_service.ChangeRepositoryName(ctx, ctx.Doer, repo, newRepoName); err != nil {
			switch {
			case repo_model.IsErrRepoAlreadyExist(err):
				ctx.Error(http.StatusUnprocessableEntity, fmt.Sprintf("repo name is already taken [name: %s]", newRepoName), err)
			case db.IsErrNameReserved(err):
				ctx.Error(http.StatusUnprocessableEntity, fmt.Sprintf("repo name is reserved [name: %s]", newRepoName), err)
			case db.IsErrNamePatternNotAllowed(err):
				ctx.Error(http.StatusUnprocessableEntity, fmt.Sprintf("repo name's pattern is not allowed [name: %s, pattern: %s]", newRepoName, err.(db.ErrNamePatternNotAllowed).Pattern), err)
			default:
				ctx.Error(http.StatusUnprocessableEntity, "ChangeRepositoryName", err)
			}
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
			if err := repo.GetBaseRepo(ctx); err != nil {
				ctx.Error(http.StatusInternalServerError, "Unable to load base repository", err)
				return err
			}
			*opts.Private = repo.BaseRepo.IsPrivate
		}

		visibilityChanged = repo.IsPrivate != *opts.Private
		// when ForcePrivate enabled, you could change public repo to private, but only admin users can change private to public
		if visibilityChanged && setting.Repository.ForcePrivate && !*opts.Private && !ctx.Doer.IsAdmin {
			err := fmt.Errorf("cannot change private repository to public")
			ctx.Error(http.StatusUnprocessableEntity, "Force Private enabled", err)
			return err
		}

		repo.IsPrivate = *opts.Private
	}

	if opts.Template != nil {
		repo.IsTemplate = *opts.Template
	}

	if ctx.Repo.GitRepo == nil && !repo.IsEmpty {
		var err error
		ctx.Repo.GitRepo, err = gitrepo.OpenRepository(ctx, repo)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "Unable to OpenRepository", err)
			return err
		}
		defer ctx.Repo.GitRepo.Close()
	}

	// Default branch only updated if changed and exist or the repository is empty
	if opts.DefaultBranch != nil && repo.DefaultBranch != *opts.DefaultBranch && (repo.IsEmpty || ctx.Repo.GitRepo.IsBranchExist(*opts.DefaultBranch)) {
		if !repo.IsEmpty {
			if err := gitrepo.SetDefaultBranch(ctx, ctx.Repo.Repository, *opts.DefaultBranch); err != nil {
				if !git.IsErrUnsupportedVersion(err) {
					ctx.Error(http.StatusInternalServerError, "SetDefaultBranch", err)
					return err
				}
			}
		}
		repo.DefaultBranch = *opts.DefaultBranch
	}

	if err := repo_service.UpdateRepository(ctx, repo, visibilityChanged); err != nil {
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

	var units []repo_model.RepoUnit
	var deleteUnitTypes []unit_model.Type

	currHasIssues := repo.UnitEnabled(ctx, unit_model.TypeIssues)
	newHasIssues := currHasIssues
	if opts.HasIssues != nil {
		newHasIssues = *opts.HasIssues
	}
	if currHasIssues || newHasIssues {
		if newHasIssues && opts.ExternalTracker != nil && !unit_model.TypeExternalTracker.UnitGlobalDisabled() {
			// Check that values are valid
			if !validation.IsValidExternalURL(opts.ExternalTracker.ExternalTrackerURL) {
				err := fmt.Errorf("External tracker URL not valid")
				ctx.Error(http.StatusUnprocessableEntity, "Invalid external tracker URL", err)
				return err
			}
			if len(opts.ExternalTracker.ExternalTrackerFormat) != 0 && !validation.IsValidExternalTrackerURLFormat(opts.ExternalTracker.ExternalTrackerFormat) {
				err := fmt.Errorf("External tracker URL format not valid")
				ctx.Error(http.StatusUnprocessableEntity, "Invalid external tracker URL format", err)
				return err
			}

			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypeExternalTracker,
				Config: &repo_model.ExternalTrackerConfig{
					ExternalTrackerURL:           opts.ExternalTracker.ExternalTrackerURL,
					ExternalTrackerFormat:        opts.ExternalTracker.ExternalTrackerFormat,
					ExternalTrackerStyle:         opts.ExternalTracker.ExternalTrackerStyle,
					ExternalTrackerRegexpPattern: opts.ExternalTracker.ExternalTrackerRegexpPattern,
				},
			})
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeIssues)
		} else if newHasIssues && opts.ExternalTracker == nil && !unit_model.TypeIssues.UnitGlobalDisabled() {
			// Default to built-in tracker
			var config *repo_model.IssuesConfig

			if opts.InternalTracker != nil {
				config = &repo_model.IssuesConfig{
					EnableTimetracker:                opts.InternalTracker.EnableTimeTracker,
					AllowOnlyContributorsToTrackTime: opts.InternalTracker.AllowOnlyContributorsToTrackTime,
					EnableDependencies:               opts.InternalTracker.EnableIssueDependencies,
				}
			} else if unit, err := repo.GetUnit(ctx, unit_model.TypeIssues); err != nil {
				// Unit type doesn't exist so we make a new config file with default values
				config = &repo_model.IssuesConfig{
					EnableTimetracker:                true,
					AllowOnlyContributorsToTrackTime: true,
					EnableDependencies:               true,
				}
			} else {
				config = unit.IssuesConfig()
			}

			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypeIssues,
				Config: config,
			})
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeExternalTracker)
		} else if !newHasIssues {
			if !unit_model.TypeExternalTracker.UnitGlobalDisabled() {
				deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeExternalTracker)
			}
			if !unit_model.TypeIssues.UnitGlobalDisabled() {
				deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeIssues)
			}
		}
	}

	currHasWiki := repo.UnitEnabled(ctx, unit_model.TypeWiki)
	newHasWiki := currHasWiki
	if opts.HasWiki != nil {
		newHasWiki = *opts.HasWiki
	}
	if currHasWiki || newHasWiki {
		if newHasWiki && opts.ExternalWiki != nil && !unit_model.TypeExternalWiki.UnitGlobalDisabled() {
			// Check that values are valid
			if !validation.IsValidExternalURL(opts.ExternalWiki.ExternalWikiURL) {
				err := fmt.Errorf("External wiki URL not valid")
				ctx.Error(http.StatusUnprocessableEntity, "", "Invalid external wiki URL")
				return err
			}

			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypeExternalWiki,
				Config: &repo_model.ExternalWikiConfig{
					ExternalWikiURL: opts.ExternalWiki.ExternalWikiURL,
				},
			})
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeWiki)
		} else if newHasWiki && opts.ExternalWiki == nil && !unit_model.TypeWiki.UnitGlobalDisabled() {
			config := &repo_model.UnitConfig{}
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypeWiki,
				Config: config,
			})
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeExternalWiki)
		} else if !newHasWiki {
			if !unit_model.TypeExternalWiki.UnitGlobalDisabled() {
				deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeExternalWiki)
			}
			if !unit_model.TypeWiki.UnitGlobalDisabled() {
				deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeWiki)
			}
		}
	}

	currHasPullRequests := repo.UnitEnabled(ctx, unit_model.TypePullRequests)
	newHasPullRequests := currHasPullRequests
	if opts.HasPullRequests != nil {
		newHasPullRequests = *opts.HasPullRequests
	}
	if currHasPullRequests || newHasPullRequests {
		if newHasPullRequests && !unit_model.TypePullRequests.UnitGlobalDisabled() {
			// We do allow setting individual PR settings through the API, so
			// we get the config settings and then set them
			// if those settings were provided in the opts.
			unit, err := repo.GetUnit(ctx, unit_model.TypePullRequests)
			var config *repo_model.PullRequestsConfig
			if err != nil {
				// Unit type doesn't exist so we make a new config file with default values
				config = &repo_model.PullRequestsConfig{
					IgnoreWhitespaceConflicts:     false,
					AllowMerge:                    true,
					AllowRebase:                   true,
					AllowRebaseMerge:              true,
					AllowSquash:                   true,
					AllowFastForwardOnly:          true,
					AllowManualMerge:              true,
					AutodetectManualMerge:         false,
					AllowRebaseUpdate:             true,
					DefaultDeleteBranchAfterMerge: false,
					DefaultMergeStyle:             repo_model.MergeStyleMerge,
					DefaultAllowMaintainerEdit:    false,
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
			if opts.AllowFastForwardOnly != nil {
				config.AllowFastForwardOnly = *opts.AllowFastForwardOnly
			}
			if opts.AllowManualMerge != nil {
				config.AllowManualMerge = *opts.AllowManualMerge
			}
			if opts.AutodetectManualMerge != nil {
				config.AutodetectManualMerge = *opts.AutodetectManualMerge
			}
			if opts.AllowRebaseUpdate != nil {
				config.AllowRebaseUpdate = *opts.AllowRebaseUpdate
			}
			if opts.DefaultDeleteBranchAfterMerge != nil {
				config.DefaultDeleteBranchAfterMerge = *opts.DefaultDeleteBranchAfterMerge
			}
			if opts.DefaultMergeStyle != nil {
				config.DefaultMergeStyle = repo_model.MergeStyle(*opts.DefaultMergeStyle)
			}
			if opts.DefaultAllowMaintainerEdit != nil {
				config.DefaultAllowMaintainerEdit = *opts.DefaultAllowMaintainerEdit
			}

			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypePullRequests,
				Config: config,
			})
		} else if !newHasPullRequests && !unit_model.TypePullRequests.UnitGlobalDisabled() {
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypePullRequests)
		}
	}

	currHasProjects := repo.UnitEnabled(ctx, unit_model.TypeProjects)
	newHasProjects := currHasProjects
	if opts.HasProjects != nil {
		newHasProjects = *opts.HasProjects
	}
	if currHasProjects || newHasProjects {
		if newHasProjects && !unit_model.TypeProjects.UnitGlobalDisabled() {
			unit, err := repo.GetUnit(ctx, unit_model.TypeProjects)
			var config *repo_model.ProjectsConfig
			if err != nil {
				config = &repo_model.ProjectsConfig{
					ProjectsMode: repo_model.ProjectsModeAll,
				}
			} else {
				config = unit.ProjectsConfig()
			}

			if opts.ProjectsMode != nil {
				config.ProjectsMode = repo_model.ProjectsMode(*opts.ProjectsMode)
			}

			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypeProjects,
				Config: config,
			})
		} else if !newHasProjects && !unit_model.TypeProjects.UnitGlobalDisabled() {
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeProjects)
		}
	}

	if opts.HasReleases != nil && !unit_model.TypeReleases.UnitGlobalDisabled() {
		if *opts.HasReleases {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypeReleases,
			})
		} else {
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeReleases)
		}
	}

	if opts.HasPackages != nil && !unit_model.TypePackages.UnitGlobalDisabled() {
		if *opts.HasPackages {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypePackages,
			})
		} else {
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypePackages)
		}
	}

	if opts.HasActions != nil && !unit_model.TypeActions.UnitGlobalDisabled() {
		if *opts.HasActions {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypeActions,
			})
		} else {
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeActions)
		}
	}

	if len(units)+len(deleteUnitTypes) > 0 {
		if err := repo_service.UpdateRepositoryUnits(ctx, repo, units, deleteUnitTypes); err != nil {
			ctx.Error(http.StatusInternalServerError, "UpdateRepositoryUnits", err)
			return err
		}
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
			if err := repo_model.SetArchiveRepoState(ctx, repo, *opts.Archived); err != nil {
				log.Error("Tried to archive a repo: %s", err)
				ctx.Error(http.StatusInternalServerError, "ArchiveRepoState", err)
				return err
			}
			if err := actions_model.CleanRepoScheduleTasks(ctx, repo); err != nil {
				log.Error("CleanRepoScheduleTasks for archived repo %s/%s: %v", ctx.Repo.Owner.Name, repo.Name, err)
			}
			log.Trace("Repository was archived: %s/%s", ctx.Repo.Owner.Name, repo.Name)
		} else {
			if err := repo_model.SetArchiveRepoState(ctx, repo, *opts.Archived); err != nil {
				log.Error("Tried to un-archive a repo: %s", err)
				ctx.Error(http.StatusInternalServerError, "ArchiveRepoState", err)
				return err
			}
			if ctx.Repo.Repository.UnitEnabled(ctx, unit_model.TypeActions) {
				if err := actions_service.DetectAndHandleSchedules(ctx, repo); err != nil {
					log.Error("DetectAndHandleSchedules for un-archived repo %s/%s: %v", ctx.Repo.Owner.Name, repo.Name, err)
				}
			}
			log.Trace("Repository was un-archived: %s/%s", ctx.Repo.Owner.Name, repo.Name)
		}
	}
	return nil
}

// updateMirror updates a repo's mirror Interval and EnablePrune
func updateMirror(ctx *context.APIContext, opts api.EditRepoOption) error {
	repo := ctx.Repo.Repository

	// Skip this update if the repo is not a mirror, do not return error.
	// Because reporting errors only makes the logic more complex&fragile, it doesn't really help end users.
	if !repo.IsMirror {
		return nil
	}

	// get the mirror from the repo
	mirror, err := repo_model.GetMirrorByRepoID(ctx, repo.ID)
	if err != nil {
		log.Error("Failed to get mirror: %s", err)
		ctx.Error(http.StatusInternalServerError, "MirrorInterval", err)
		return err
	}

	// update MirrorInterval
	if opts.MirrorInterval != nil {
		// MirrorInterval should be a duration
		interval, err := time.ParseDuration(*opts.MirrorInterval)
		if err != nil {
			log.Error("Wrong format for MirrorInternal Sent: %s", err)
			ctx.Error(http.StatusUnprocessableEntity, "MirrorInterval", err)
			return err
		}

		// Ensure the provided duration is not too short
		if interval != 0 && interval < setting.Mirror.MinInterval {
			err := fmt.Errorf("invalid mirror interval: %s is below minimum interval: %s", interval, setting.Mirror.MinInterval)
			ctx.Error(http.StatusUnprocessableEntity, "MirrorInterval", err)
			return err
		}

		mirror.Interval = interval
		mirror.Repo = repo
		mirror.ScheduleNextUpdate()
		log.Trace("Repository %s Mirror[%d] Set Interval: %s NextUpdateUnix: %s", repo.FullName(), mirror.ID, interval, mirror.NextUpdateUnix)
	}

	// update EnablePrune
	if opts.EnablePrune != nil {
		mirror.EnablePrune = *opts.EnablePrune
		log.Trace("Repository %s Mirror[%d] Set EnablePrune: %t", repo.FullName(), mirror.ID, mirror.EnablePrune)
	}

	// finally update the mirror in the DB
	if err := repo_model.UpdateMirror(ctx, mirror); err != nil {
		log.Error("Failed to Set Mirror Interval: %s", err)
		ctx.Error(http.StatusUnprocessableEntity, "MirrorInterval", err)
		return err
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	owner := ctx.Repo.Owner
	repo := ctx.Repo.Repository

	canDelete, err := repo_module.CanUserDelete(ctx, repo, ctx.Doer)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CanUserDelete", err)
		return
	} else if !canDelete {
		ctx.Error(http.StatusForbidden, "", "Given user is not owner of organization.")
		return
	}

	if ctx.Repo.GitRepo != nil {
		ctx.Repo.GitRepo.Close()
	}

	if err := repo_service.DeleteRepository(ctx, ctx.Doer, repo, true); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteRepository", err)
		return
	}

	log.Trace("Repository deleted: %s/%s", owner.Name, repo.Name)
	ctx.Status(http.StatusNoContent)
}

// GetIssueTemplates returns the issue templates for a repository
func GetIssueTemplates(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issue_templates repository repoGetIssueTemplates
	// ---
	// summary: Get available issue templates for a repository
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
	//     "$ref": "#/responses/IssueTemplates"
	//   "404":
	//     "$ref": "#/responses/notFound"
	ret := issue.ParseTemplatesFromDefaultBranch(ctx.Repo.Repository, ctx.Repo.GitRepo)
	if cnt := len(ret.TemplateErrors); cnt != 0 {
		ctx.Resp.Header().Add("X-Gitea-Warning", "error occurs when parsing issue template: count="+strconv.Itoa(cnt))
	}
	ctx.JSON(http.StatusOK, ret.IssueTemplates)
}

// GetIssueConfig returns the issue config for a repo
func GetIssueConfig(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issue_config repository repoGetIssueConfig
	// ---
	// summary: Returns the issue config for a repo
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
	//     "$ref": "#/responses/RepoIssueConfig"
	//   "404":
	//     "$ref": "#/responses/notFound"
	issueConfig, _ := issue.GetTemplateConfigFromDefaultBranch(ctx.Repo.Repository, ctx.Repo.GitRepo)
	ctx.JSON(http.StatusOK, issueConfig)
}

// ValidateIssueConfig returns validation errors for the issue config
func ValidateIssueConfig(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issue_config/validate repository repoValidateIssueConfig
	// ---
	// summary: Returns the validation information for a issue config
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
	//     "$ref": "#/responses/RepoIssueConfigValidation"
	//   "404":
	//     "$ref": "#/responses/notFound"
	_, err := issue.GetTemplateConfigFromDefaultBranch(ctx.Repo.Repository, ctx.Repo.GitRepo)

	if err == nil {
		ctx.JSON(http.StatusOK, api.IssueConfigValidation{Valid: true, Message: ""})
	} else {
		ctx.JSON(http.StatusOK, api.IssueConfigValidation{Valid: false, Message: err.Error()})
	}
}

func ListRepoActivityFeeds(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/activities/feeds repository repoListActivityFeeds
	// ---
	// summary: List a repository's activity feeds
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
	// - name: date
	//   in: query
	//   description: the date of the activities to be found
	//   type: string
	//   format: date
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityFeedsList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	listOptions := utils.GetListOptions(ctx)

	opts := activities_model.GetFeedsOptions{
		RequestedRepo:  ctx.Repo.Repository,
		Actor:          ctx.Doer,
		IncludePrivate: true,
		Date:           ctx.FormString("date"),
		ListOptions:    listOptions,
	}

	feeds, count, err := activities_model.GetFeeds(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetFeeds", err)
		return
	}
	ctx.SetTotalCountHeader(count)

	ctx.JSON(http.StatusOK, convert.ToActivities(ctx, feeds, ctx.Doer))
}
