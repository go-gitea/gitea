// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	unit_model "code.gitea.io/gitea/models/unit"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// ListEnvironments list environments for a repository
func ListEnvironments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/environments repository listEnvironments
	// ---
	// summary: List environments for a repository
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
	//   description: name of the repository
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
	//     "$ref": "#/responses/EnvironmentList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !ctx.Repo.CanRead(unit_model.TypeActions) {
		ctx.APIError(http.StatusForbidden, "no permission to access actions")
		return
	}

	listOptions := utils.GetListOptions(ctx)
	
	opts := actions_model.FindEnvironmentsOptions{
		ListOptions: listOptions,
		RepoID:      ctx.Repo.Repository.ID,
	}

	environments, err := actions_model.FindEnvironments(ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	count, err := actions_model.CountEnvironments(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiEnvironments := make([]*api.Environment, len(environments))
	for i, env := range environments {
		apiEnv, err := convert.ToEnvironment(ctx, env)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		apiEnvironments[i] = apiEnv
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, &api.EnvironmentListResponse{
		Environments: apiEnvironments,
		TotalCount:   count,
	})
}

// GetEnvironment get a single environment
func GetEnvironment(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/environments/{environment_name} repository getEnvironment
	// ---
	// summary: Get a specific environment
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
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: environment_name
	//   in: path
	//   description: name of the environment
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Environment"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !ctx.Repo.CanRead(unit_model.TypeActions) {
		ctx.APIError(http.StatusForbidden, "no permission to access actions")
		return
	}

	environmentName := ctx.PathParam("environment_name")
	env, err := actions_model.GetEnvironmentByRepoIDAndName(ctx, ctx.Repo.Repository.ID, environmentName)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	apiEnv, err := convert.ToEnvironment(ctx, env)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, apiEnv)
}

// CreateOrUpdateEnvironment create or update an environment
func CreateOrUpdateEnvironment(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/environments/{environment_name} repository createOrUpdateEnvironment
	// ---
	// summary: Create or update a deployment environment
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
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: environment_name
	//   in: path
	//   description: name of the environment
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateEnvironmentOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Environment"
	//   "201":
	//     "$ref": "#/responses/Environment"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	if !ctx.Repo.CanWrite(unit_model.TypeActions) {
		ctx.APIError(http.StatusForbidden, "no permission to create environments")
		return
	}

	environmentName := ctx.PathParam("environment_name")
	form := web.GetForm(ctx).(*api.CreateEnvironmentOption)

	// Override the name from the path parameter
	form.Name = environmentName

	// Check if environment already exists
	existingEnv, err := actions_model.GetEnvironmentByRepoIDAndName(ctx, ctx.Repo.Repository.ID, environmentName)
	isUpdate := err == nil

	if isUpdate {
		// Update existing environment
		opts := actions_model.UpdateEnvironmentOptions{}
		if form.Description != "" {
			opts.Description = &form.Description
		}
		if form.ExternalURL != "" {
			opts.ExternalURL = &form.ExternalURL
		}
		if form.ProtectionRules != "" {
			opts.ProtectionRules = &form.ProtectionRules
		}

		if err := actions_model.UpdateEnvironment(ctx, existingEnv, opts); err != nil {
			ctx.APIErrorInternal(err)
			return
		}

		// Reload the environment to get updated data
		env, err := actions_model.GetEnvironmentByRepoIDAndName(ctx, ctx.Repo.Repository.ID, environmentName)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}

		apiEnv, err := convert.ToEnvironment(ctx, env)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}

		ctx.JSON(http.StatusOK, apiEnv)
	} else {
		// Create new environment
		createOpts := actions_model.CreateEnvironmentOptions{
			RepoID:          ctx.Repo.Repository.ID,
			Name:            form.Name,
			Description:     form.Description,
			ExternalURL:     form.ExternalURL,
			ProtectionRules: form.ProtectionRules,
			CreatedByID:     ctx.Doer.ID,
		}

		env, err := actions_model.CreateEnvironment(ctx, createOpts)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}

		apiEnv, err := convert.ToEnvironment(ctx, env)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}

		ctx.JSON(http.StatusCreated, apiEnv)
	}
}

// DeleteEnvironment delete an environment
func DeleteEnvironment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/environments/{environment_name} repository deleteEnvironment
	// ---
	// summary: Delete a deployment environment
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
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: environment_name
	//   in: path
	//   description: name of the environment
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !ctx.Repo.CanWrite(unit_model.TypeActions) {
		ctx.APIError(http.StatusForbidden, "no permission to delete environments")
		return
	}

	environmentName := ctx.PathParam("environment_name")

	// Check if environment exists
	_, err := actions_model.GetEnvironmentByRepoIDAndName(ctx, ctx.Repo.Repository.ID, environmentName)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if err := actions_model.DeleteEnvironment(ctx, ctx.Repo.Repository.ID, environmentName); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}