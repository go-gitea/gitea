// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/optional"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	repo_service "code.gitea.io/gitea/services/repository"
)

// ListForks list a repository's forks
func ListForks(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/forks repository listForks
	// ---
	// summary: List a repository's forks
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
	//     "$ref": "#/responses/RepositoryList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	forks, total, err := repo_service.FindForks(ctx, ctx.Repo.Repository, ctx.Doer, utils.GetListOptions(ctx))
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if err := repo_model.RepositoryList(forks).LoadOwners(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if err := repo_model.RepositoryList(forks).LoadUnits(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiForks := make([]*api.Repository, len(forks))
	for i, fork := range forks {
		permission, err := access_model.GetDoerRepoPermission(ctx, fork, ctx.Doer)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		apiForks[i] = convert.ToRepo(ctx, fork, permission)
	}

	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, apiForks)
}

func prepareDoerCreateRepoInOrg(ctx *context.APIContext, orgName string) *organization.Organization {
	org, err := organization.GetOrgByName(ctx, orgName)
	if errors.Is(err, util.ErrNotExist) {
		ctx.APIErrorNotFound()
		return nil
	} else if err != nil {
		ctx.APIErrorInternal(err)
		return nil
	}

	if !organization.HasOrgOrUserVisible(ctx, org.AsUser(), ctx.Doer) {
		ctx.APIErrorNotFound()
		return nil
	}

	if !ctx.Doer.IsAdmin {
		canCreate, err := org.CanCreateOrgRepo(ctx, ctx.Doer.ID)
		if err != nil {
			ctx.APIErrorInternal(err)
			return nil
		}
		if !canCreate {
			ctx.APIError(http.StatusForbidden, "User is not allowed to create repositories in this organization.")
			return nil
		}
	}
	return org
}

// CreateFork create a fork of a repo
func CreateFork(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/forks repository createFork
	// ---
	// summary: Fork a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to fork
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to fork
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateForkOption"
	// responses:
	//   "202":
	//     "$ref": "#/responses/Repository"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     description: The repository with the same name already exists.
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.CreateForkOption)
	forkOwner := ctx.Doer // user/org that will own the fork
	if form.Organization != nil {
		org := prepareDoerCreateRepoInOrg(ctx, *form.Organization)
		if ctx.Written() {
			return
		}
		forkOwner = org.AsUser()
	}

	repo := ctx.Repo.Repository
	name := optional.FromPtr(form.Name).ValueOrDefault(repo.Name)
	fork, err := repo_service.ForkRepository(ctx, ctx.Doer, forkOwner, repo_service.ForkRepoOptions{
		BaseRepo:    repo,
		Name:        name,
		Description: repo.Description,
	})
	if err != nil {
		if errors.Is(err, util.ErrAlreadyExist) || repo_model.IsErrReachLimitOfRepo(err) {
			ctx.APIError(http.StatusConflict, err)
		} else if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.APIError(http.StatusForbidden, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	// TODO change back to 201
	ctx.JSON(http.StatusAccepted, convert.ToRepo(ctx, fork, access_model.Permission{AccessMode: perm.AccessModeOwner}))
}
