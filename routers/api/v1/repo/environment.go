// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"strings"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	secret_model "gitea.dev/models/secret"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/utils"
	actions_service "gitea.dev/services/actions"
	"gitea.dev/services/context"
)

// ListEnvironments lists all environments for a repo
func ListEnvironments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/environments repository listEnvironments
	// ---
	// summary: List environments for a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   type: integer
	// - name: limit
	//   in: query
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/EnvironmentList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	listOptions := utils.GetListOptions(ctx)
	envs, count, err := db.FindAndCount[actions_model.ActionEnvironment](ctx, actions_model.FindEnvironmentsOptions{
		RepoID:      ctx.Repo.Repository.ID,
		ListOptions: listOptions,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	result := make([]*api.ActionEnvironment, len(envs))
	for i, e := range envs {
		result[i] = toAPIEnvironment(e)
	}
	ctx.SetLinkHeader(count, listOptions.PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, result)
}

// GetEnvironment gets one environment
func GetEnvironment(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/environments/{environment_name} repository getEnvironment
	// ---
	// summary: Get a deployment environment
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   type: string
	//   required: true
	// - name: environment_name
	//   in: path
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Environment"
	//   "404":
	//     "$ref": "#/responses/notFound"

	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, ctx.PathParam("environment_name"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.JSON(http.StatusOK, toAPIEnvironment(env))
}

// CreateEnvironment creates a new environment
func CreateEnvironment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/environments repository createEnvironment
	// ---
	// summary: Create a deployment environment
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateEnvironmentOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Environment"
	//   "400":
	//     "$ref": "#/responses/error"

	opt := web.GetForm(ctx).(*api.CreateEnvironmentOption)
	env, err := actions_service.CreateEnvironment(ctx, ctx.Repo.Repository.ID, opt.Name, opt.ProtectedBranches)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusBadRequest, err.Error())
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	ctx.JSON(http.StatusCreated, toAPIEnvironment(env))
}

// UpdateEnvironment updates an environment
func UpdateEnvironment(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/environments/{environment_name} repository updateEnvironment
	// ---
	// summary: Update a deployment environment
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   type: string
	//   required: true
	// - name: environment_name
	//   in: path
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/UpdateEnvironmentOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Environment"
	//   "404":
	//     "$ref": "#/responses/notFound"

	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, ctx.PathParam("environment_name"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	opt := web.GetForm(ctx).(*api.UpdateEnvironmentOption)
	updated, err := actions_service.UpdateEnvironment(ctx, ctx.Repo.Repository.ID, env.ID, opt.Name, opt.ProtectedBranches)
	if err != nil {
		if errors.Is(err, util.ErrAlreadyExist) {
			ctx.APIError(http.StatusConflict, err.Error())
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	ctx.JSON(http.StatusOK, toAPIEnvironment(updated))
}

// DeleteEnvironment deletes an environment
func DeleteEnvironment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/environments/{environment_name} repository deleteEnvironment
	// ---
	// summary: Delete a deployment environment
	// parameters:
	// - name: owner
	//   in: path
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   type: string
	//   required: true
	// - name: environment_name
	//   in: path
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: No Content
	//   "404":
	//     "$ref": "#/responses/notFound"

	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, ctx.PathParam("environment_name"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	if err := actions_service.DeleteEnvironment(ctx, ctx.Repo.Repository.ID, env.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// ListEnvSecrets lists secrets for an environment
func ListEnvSecrets(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/environments/{environment_name}/secrets repository listEnvSecrets
	// ---
	// summary: List secrets for a deployment environment
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   type: string
	//   required: true
	// - name: environment_name
	//   in: path
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   type: integer
	// - name: limit
	//   in: query
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/SecretList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, ctx.PathParam("environment_name"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	listOptions := utils.GetListOptions(ctx)
	secrets, count, err := db.FindAndCount[secret_model.Secret](ctx, secret_model.FindSecretsOptions{
		RepoID:        ctx.Repo.Repository.ID,
		EnvironmentID: env.ID,
		ListOptions:   listOptions,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	result := make([]*api.Secret, len(secrets))
	for i, s := range secrets {
		result[i] = &api.Secret{
			Name:        s.Name,
			Description: s.Description,
			Created:     s.CreatedUnix.AsTime(),
		}
	}
	ctx.SetLinkHeader(count, listOptions.PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, result)
}

// CreateOrUpdateEnvSecret creates or updates an environment secret
func CreateOrUpdateEnvSecret(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/environments/{environment_name}/secrets/{secretname} repository createOrUpdateEnvSecret
	// ---
	// summary: Create or update a secret for a deployment environment
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   type: string
	//   required: true
	// - name: environment_name
	//   in: path
	//   type: string
	//   required: true
	// - name: secretname
	//   in: path
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateOrUpdateSecretOption"
	// responses:
	//   "201":
	//     description: secret created
	//   "204":
	//     description: secret updated
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, ctx.PathParam("environment_name"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	opt := web.GetForm(ctx).(*api.CreateOrUpdateSecretOption)
	_, created, err := actions_service.CreateOrUpdateEnvSecret(ctx, ctx.Repo.Repository.ID, env.ID, ctx.PathParam("secretname"), opt.Data, opt.Description)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusBadRequest, err.Error())
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	if created {
		ctx.Status(http.StatusCreated)
	} else {
		ctx.Status(http.StatusNoContent)
	}
}

// DeleteEnvSecret deletes an environment secret
func DeleteEnvSecret(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/environments/{environment_name}/secrets/{secretname} repository deleteEnvSecret
	// ---
	// summary: Delete a secret from a deployment environment
	// parameters:
	// - name: owner
	//   in: path
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   type: string
	//   required: true
	// - name: environment_name
	//   in: path
	//   type: string
	//   required: true
	// - name: secretname
	//   in: path
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: No Content
	//   "404":
	//     "$ref": "#/responses/notFound"

	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, ctx.PathParam("environment_name"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	if err := actions_service.DeleteEnvSecret(ctx, ctx.Repo.Repository.ID, env.ID, ctx.PathParam("secretname")); err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}

// ListEnvVariables lists variables for an environment
func ListEnvVariables(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/environments/{environment_name}/variables repository listEnvVariables
	// ---
	// summary: List variables for a deployment environment
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   type: string
	//   required: true
	// - name: environment_name
	//   in: path
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/VariableList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, ctx.PathParam("environment_name"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	listOptions := utils.GetListOptions(ctx)
	vars, count, err := db.FindAndCount[actions_model.ActionVariable](ctx, actions_model.FindVariablesOpts{
		RepoID:        ctx.Repo.Repository.ID,
		EnvironmentID: env.ID,
		ListOptions:   listOptions,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	result := make([]*api.ActionVariable, len(vars))
	for i, v := range vars {
		result[i] = &api.ActionVariable{
			RepoID:      v.RepoID,
			Name:        v.Name,
			Data:        v.Data,
			Description: v.Description,
		}
	}
	ctx.SetLinkHeader(count, listOptions.PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, result)
}

// CreateEnvVariable creates a variable for an environment
func CreateEnvVariable(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/environments/{environment_name}/variables/{variablename} repository createEnvVariable
	// ---
	// summary: Create a variable for a deployment environment
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   type: string
	//   required: true
	// - name: environment_name
	//   in: path
	//   type: string
	//   required: true
	// - name: variablename
	//   in: path
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateVariableOption"
	// responses:
	//   "201":
	//     description: variable created
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     description: variable already exists

	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, ctx.PathParam("environment_name"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	opt := web.GetForm(ctx).(*api.CreateVariableOption)
	variableName := strings.ToUpper(ctx.PathParam("variablename"))

	if _, err := actions_service.CreateEnvVariable(ctx, ctx.Repo.Repository.ID, env.ID, variableName, opt.Value, opt.Description); err != nil {
		if errors.Is(err, util.ErrAlreadyExist) {
			ctx.APIError(http.StatusConflict, err.Error())
		} else if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusBadRequest, err.Error())
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	ctx.Status(http.StatusCreated)
}

// UpdateEnvVariable updates an environment variable
func UpdateEnvVariable(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/environments/{environment_name}/variables/{variablename} repository updateEnvVariable
	// ---
	// summary: Update a variable for a deployment environment
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   type: string
	//   required: true
	// - name: environment_name
	//   in: path
	//   type: string
	//   required: true
	// - name: variablename
	//   in: path
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/UpdateVariableOption"
	// responses:
	//   "204":
	//     description: variable updated
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, ctx.PathParam("environment_name"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	vars, err := db.Find[actions_model.ActionVariable](ctx, actions_model.FindVariablesOpts{
		RepoID:        ctx.Repo.Repository.ID,
		EnvironmentID: env.ID,
		Name:          ctx.PathParam("variablename"),
	})
	if err != nil || len(vars) == 0 {
		ctx.APIErrorNotFound()
		return
	}
	opt := web.GetForm(ctx).(*api.UpdateVariableOption)
	if _, err := actions_service.UpdateEnvVariable(ctx, ctx.Repo.Repository.ID, env.ID, vars[0].ID, opt.Name, opt.Value, opt.Description); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// DeleteEnvVariable deletes an environment variable
func DeleteEnvVariable(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/environments/{environment_name}/variables/{variablename} repository deleteEnvVariable
	// ---
	// summary: Delete a variable from a deployment environment
	// parameters:
	// - name: owner
	//   in: path
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   type: string
	//   required: true
	// - name: environment_name
	//   in: path
	//   type: string
	//   required: true
	// - name: variablename
	//   in: path
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: No Content
	//   "404":
	//     "$ref": "#/responses/notFound"

	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, ctx.PathParam("environment_name"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	vars, err := db.Find[actions_model.ActionVariable](ctx, actions_model.FindVariablesOpts{
		RepoID:        ctx.Repo.Repository.ID,
		EnvironmentID: env.ID,
		Name:          ctx.PathParam("variablename"),
	})
	if err != nil || len(vars) == 0 {
		ctx.APIErrorNotFound()
		return
	}
	if err := actions_service.DeleteEnvVariable(ctx, ctx.Repo.Repository.ID, env.ID, vars[0].ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func toAPIEnvironment(e *actions_model.ActionEnvironment) *api.ActionEnvironment {
	return &api.ActionEnvironment{
		ID:                e.ID,
		Name:              e.Name,
		ProtectedBranches: e.ProtectedBranches,
		CreatedAt:         e.CreatedUnix.AsTime(),
		UpdatedAt:         e.UpdatedUnix.AsTime(),
	}
}
