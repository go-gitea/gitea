// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	actions_model "gitea.dev/models/actions"
	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	secret_model "gitea.dev/models/secret"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/utils"
	actions_service "gitea.dev/services/actions"
	"gitea.dev/services/audit"
	"gitea.dev/services/context"
	secret_service "gitea.dev/services/secrets"
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	env, ok := getEnvironmentOrError(ctx)
	if !ok {
		return
	}
	ctx.JSON(http.StatusOK, toAPIEnvironment(env))
}

// CreateOrUpdateEnvironment creates or replaces a deployment environment
func CreateOrUpdateEnvironment(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/environments/{environment_name} repository createOrUpdateEnvironment
	// ---
	// summary: Create or update a deployment environment
	// consumes:
	// - application/json
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
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CreateOrUpdateEnvironmentOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Environment"
	//   "201":
	//     "$ref": "#/responses/Environment"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opt := web.GetForm[*api.CreateOrUpdateEnvironmentOption](ctx)
	env, created, err := actions_service.GetOrCreateEnvironment(ctx, ctx.Repo.Repository.ID, ctx.PathParam("environment_name"), opt.AllowedBranchPatterns)
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	if created {
		audit.Record(ctx, audit_model.RepositoryEnvironmentAdd, ctx.Repo.Repository, "environment", env.Name)
	} else {
		changed, err := actions_service.UpdateEnvironment(ctx, env, opt.AllowedBranchPatterns)
		if err != nil {
			ctx.APIErrorAuto(err)
			return
		}
		if changed {
			audit.Record(ctx, audit_model.RepositoryEnvironmentUpdate, ctx.Repo.Repository, "environment", env.Name)
		}
	}
	ctx.JSON(util.Iif(created, http.StatusCreated, http.StatusOK), toAPIEnvironment(env))
}

// DeleteEnvironment deletes an environment
func DeleteEnvironment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/environments/{environment_name} repository deleteEnvironment
	// ---
	// summary: Delete a deployment environment
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
	//     description: No Content
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	env, ok := getEnvironmentOrError(ctx)
	if !ok {
		return
	}
	if err := actions_model.DeleteEnvironment(ctx, ctx.Repo.Repository.ID, env.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	audit.Record(ctx, audit_model.RepositoryEnvironmentRemove, ctx.Repo.Repository, "environment", env.Name)
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
	//     "$ref": "#/responses/SecretList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	env, ok := getEnvironmentOrError(ctx)
	if !ok {
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
	// - name: secretname
	//   in: path
	//   description: name of the secret
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
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	env, ok := getEnvironmentOrError(ctx)
	if !ok {
		return
	}
	opt := web.GetForm[*api.CreateOrUpdateSecretOption](ctx)
	s, created, err := secret_service.CreateOrUpdateSecret(ctx, 0, ctx.Repo.Repository.ID, env.ID, ctx.PathParam("secretname"), opt.Data, opt.Description)
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	action := audit.SecretUpdate
	if created {
		action = audit.SecretAdd
	}
	audit.RecordScoped(ctx, nil, ctx.Repo.Repository, action, "secret", s.Name, "environment", env.Name)
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
	// - name: secretname
	//   in: path
	//   description: name of the secret
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: No Content
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	env, ok := getEnvironmentOrError(ctx)
	if !ok {
		return
	}
	s, err := secret_service.DeleteSecretByName(ctx, 0, ctx.Repo.Repository.ID, env.ID, ctx.PathParam("secretname"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	audit.RecordScoped(ctx, nil, ctx.Repo.Repository, audit.SecretRemove, "secret", s.Name, "environment", env.Name)
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
	//     "$ref": "#/responses/VariableList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	env, ok := getEnvironmentOrError(ctx)
	if !ok {
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
	// - name: variablename
	//   in: path
	//   description: name of the variable
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
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     description: variable already exists

	env, ok := getEnvironmentOrError(ctx)
	if !ok {
		return
	}
	opt := web.GetForm[*api.CreateVariableOption](ctx)
	variableName := ctx.PathParam("variablename")

	_, err := actions_service.GetVariable(ctx, actions_model.FindVariablesOpts{RepoID: ctx.Repo.Repository.ID, EnvironmentID: env.ID, Name: variableName})
	if err == nil {
		ctx.APIError(http.StatusConflict, "variable name already exists")
		return
	}
	if !errors.Is(err, util.ErrNotExist) {
		ctx.APIErrorInternal(err)
		return
	}

	if _, err := actions_service.CreateVariable(ctx, 0, ctx.Repo.Repository.ID, env.ID, variableName, opt.Value, opt.Description); err != nil {
		ctx.APIErrorAuto(err)
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
	// - name: variablename
	//   in: path
	//   description: name of the variable
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
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	env, ok := getEnvironmentOrError(ctx)
	if !ok {
		return
	}
	v, err := actions_service.GetVariable(ctx, actions_model.FindVariablesOpts{RepoID: ctx.Repo.Repository.ID, EnvironmentID: env.ID, Name: ctx.PathParam("variablename")})
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	opt := web.GetForm[*api.UpdateVariableOption](ctx)
	if opt.Name != "" {
		v.Name = opt.Name
	}
	v.Data = opt.Value
	v.Description = opt.Description
	if _, err := actions_service.UpdateVariableNameData(ctx, v); err != nil {
		ctx.APIErrorAuto(err)
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
	// - name: variablename
	//   in: path
	//   description: name of the variable
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: No Content
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	env, ok := getEnvironmentOrError(ctx)
	if !ok {
		return
	}
	if err := actions_service.DeleteVariableByName(ctx, 0, ctx.Repo.Repository.ID, env.ID, ctx.PathParam("variablename")); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// getEnvironmentOrError loads the environment named by the request path, writing the API error
// response and returning ok=false if it cannot be found.
func getEnvironmentOrError(ctx *context.APIContext) (env *actions_model.ActionEnvironment, ok bool) {
	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, ctx.PathParam("environment_name"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return nil, false
	}
	return env, true
}

func toAPIEnvironment(e *actions_model.ActionEnvironment) *api.ActionEnvironment {
	return &api.ActionEnvironment{
		ID:                    e.ID,
		Name:                  e.Name,
		AllowedBranchPatterns: e.BranchPatterns(),
		CreatedAt:             e.CreatedUnix.AsTime(),
		UpdatedAt:             e.UpdatedUnix.AsTime(),
	}
}
