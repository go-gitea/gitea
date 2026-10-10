// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	secret_model "gitea.dev/models/secret"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/shared"
	"gitea.dev/routers/api/v1/utils"
	actions_service "gitea.dev/services/actions"
	"gitea.dev/services/audit"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	secret_service "gitea.dev/services/secrets"
)

// ListActionsSecrets list a repo's actions secrets
func (Action) ListActionsSecrets(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/secrets repository repoListActionsSecrets
	// ---
	// summary: List a repo's actions secrets
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
	//     "$ref": "#/responses/SecretList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repo := ctx.Repo.Repository
	listOptions := utils.GetListOptions(ctx)

	opts := &secret_model.FindSecretsOptions{
		RepoID:      repo.ID,
		ListOptions: listOptions,
	}

	secrets, count, err := db.FindAndCount[secret_model.Secret](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiSecrets := make([]*api.Secret, len(secrets))
	for k, v := range secrets {
		apiSecrets[k] = &api.Secret{
			Name:        v.Name,
			Description: v.Description,
			Created:     v.CreatedUnix.AsTime(),
		}
	}
	ctx.SetLinkHeader(count, listOptions.PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, apiSecrets)
}

// create or update one secret of the repository
func (Action) CreateOrUpdateSecret(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/actions/secrets/{secretname} repository updateRepoSecret
	// ---
	// summary: Create or Update a secret value in a repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repository
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
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
	//     description: response when creating a secret
	//   "204":
	//     description: response when updating a secret
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repo := ctx.Repo.Repository

	opt := web.GetForm[*api.CreateOrUpdateSecretOption](ctx)

	s, created, err := secret_service.CreateOrUpdateSecret(ctx, 0, repo.ID, ctx.PathParam("secretname"), opt.Data, opt.Description)
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	actions := audit.SecretUpdate
	if created {
		actions = audit.SecretAdd
	}
	audit.RecordScoped(ctx, nil, repo, actions, "secret", s.Name)

	if created {
		ctx.Status(http.StatusCreated)
	} else {
		ctx.Status(http.StatusNoContent)
	}
}

// DeleteSecret delete one secret of the repository
func (Action) DeleteSecret(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/actions/secrets/{secretname} repository deleteRepoSecret
	// ---
	// summary: Delete a secret in a repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repository
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: secretname
	//   in: path
	//   description: name of the secret
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: delete one secret of the repository
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repo := ctx.Repo.Repository

	s, err := secret_service.DeleteSecretByName(ctx, 0, repo.ID, ctx.PathParam("secretname"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	audit.RecordScoped(ctx, nil, repo, audit.SecretRemove, "secret", s.Name)

	ctx.Status(http.StatusNoContent)
}

// GetVariable get a repo-level variable
func (Action) GetVariable(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/variables/{variablename} repository getRepoVariable
	// ---
	// summary: Get a repo-level variable
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
	// - name: variablename
	//   in: path
	//   description: name of the variable
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//			"$ref": "#/responses/ActionVariable"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	v, err := actions_service.GetVariable(ctx, actions_model.FindVariablesOpts{
		RepoID: ctx.Repo.Repository.ID,
		Name:   ctx.PathParam("variablename"),
	})
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(http.StatusNotFound, err.Error())
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	variable := &api.ActionVariable{
		OwnerID:     v.OwnerID,
		RepoID:      v.RepoID,
		Name:        v.Name,
		Data:        v.Data,
		Description: v.Description,
	}

	ctx.JSON(http.StatusOK, variable)
}

// DeleteVariable delete a repo-level variable
func (Action) DeleteVariable(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/actions/variables/{variablename} repository deleteRepoVariable
	// ---
	// summary: Delete a repo-level variable
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
	// - name: variablename
	//   in: path
	//   description: name of the variable
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//			"$ref": "#/responses/ActionVariable"
	//   "201":
	//     description: response when deleting a variable
	//   "204":
	//     description: response when deleting a variable
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if err := actions_service.DeleteVariableByName(ctx, 0, ctx.Repo.Repository.ID, ctx.PathParam("variablename")); err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// CreateVariable create a repo-level variable
func (Action) CreateVariable(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/variables/{variablename} repository createRepoVariable
	// ---
	// summary: Create a repo-level variable
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
	//     description: response when creating a repo-level variable
	//   "400":
	//     "$ref": "#/responses/error"
	//   "409":
	//     description: variable name already exists.
	//   "500":
	//     "$ref": "#/responses/error"

	opt := web.GetForm[*api.CreateVariableOption](ctx)

	repoID := ctx.Repo.Repository.ID
	variableName := ctx.PathParam("variablename")

	v, err := actions_service.GetVariable(ctx, actions_model.FindVariablesOpts{
		RepoID: repoID,
		Name:   variableName,
	})
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		ctx.APIErrorInternal(err)
		return
	}
	if v != nil && v.ID > 0 {
		ctx.APIError(http.StatusConflict, "variable name already exists")
		return
	}

	if _, err := actions_service.CreateVariable(ctx, 0, repoID, variableName, opt.Value, opt.Description); err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	ctx.Status(http.StatusCreated)
}

// UpdateVariable update a repo-level variable
func (Action) UpdateVariable(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/actions/variables/{variablename} repository updateRepoVariable
	// ---
	// summary: Update a repo-level variable
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
	//   "201":
	//     description: response when updating a repo-level variable
	//   "204":
	//     description: response when updating a repo-level variable
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opt := web.GetForm[*api.UpdateVariableOption](ctx)

	v, err := actions_service.GetVariable(ctx, actions_model.FindVariablesOpts{
		RepoID: ctx.Repo.Repository.ID,
		Name:   ctx.PathParam("variablename"),
	})
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	if opt.Name == "" {
		opt.Name = ctx.PathParam("variablename")
	}

	v.Name = opt.Name
	v.Data = opt.Value
	v.Description = opt.Description

	if _, err := actions_service.UpdateVariableNameData(ctx, v); err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// ListVariables list repo-level variables
func (Action) ListVariables(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/variables repository getRepoVariablesList
	// ---
	// summary: Get repo-level variables list
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
	//		 "$ref": "#/responses/VariableList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	listOptions := utils.GetListOptions(ctx)

	vars, count, err := db.FindAndCount[actions_model.ActionVariable](ctx, &actions_model.FindVariablesOpts{
		RepoID:      ctx.Repo.Repository.ID,
		ListOptions: listOptions,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	variables := make([]*api.ActionVariable, len(vars))
	for i, v := range vars {
		variables[i] = &api.ActionVariable{
			OwnerID:     v.OwnerID,
			RepoID:      v.RepoID,
			Name:        v.Name,
			Data:        v.Data,
			Description: v.Description,
		}
	}

	ctx.SetLinkHeader(count, listOptions.PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, variables)
}

// CreateRegistrationToken returns the token to register repo runners
func (Action) CreateRegistrationToken(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/runners/registration-token repository repoCreateRunnerRegistrationToken
	// ---
	// summary: Get a repository's actions runner registration token
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
	//     "$ref": "#/responses/RegistrationToken"

	shared.GetRegistrationToken(ctx, 0, ctx.Repo.Repository.ID)
}

// ListRunners get repo-level runners
func (Action) ListRunners(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runners repository getRepoRunners
	// ---
	// summary: Get repo-level runners
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
	// - name: disabled
	//   in: query
	//   description: filter by disabled status (true or false)
	//   type: boolean
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/responses/RunnerList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	shared.ListRunners(ctx, 0, ctx.Repo.Repository.ID)
}

// GetRunner get a repo-level runner
func (Action) GetRunner(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runners/{runner_id} repository getRepoRunner
	// ---
	// summary: Get a repo-level runner
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
	// - name: runner_id
	//   in: path
	//   description: id of the runner
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Runner"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	shared.GetRunner(ctx, 0, ctx.Repo.Repository.ID, ctx.PathParamInt64("runner_id"))
}

// DeleteRunner delete a repo-level runner
func (Action) DeleteRunner(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/actions/runners/{runner_id} repository deleteRepoRunner
	// ---
	// summary: Delete a repo-level runner
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
	// - name: runner_id
	//   in: path
	//   description: id of the runner
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: runner has been deleted
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	shared.DeleteRunner(ctx, 0, ctx.Repo.Repository.ID, ctx.PathParamInt64("runner_id"))
}

// UpdateRunner update a repo-level runner
func (Action) UpdateRunner(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/actions/runners/{runner_id} repository updateRepoRunner
	// ---
	// summary: Update a repo-level runner
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
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: runner_id
	//   in: path
	//   description: id of the runner
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditActionRunnerOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Runner"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	shared.UpdateRunner(ctx, 0, ctx.Repo.Repository.ID, ctx.PathParamInt64("runner_id"))
}

// ListWorkflowJobs Lists all jobs for a repository.
func (Action) ListWorkflowJobs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/jobs repository listWorkflowJobs
	// ---
	// summary: Lists all jobs for a repository
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
	// - name: status
	//   in: query
	//   description: workflow status (requested, pending, queued, in_progress, failure, success, skipped)
	//   type: string
	//   required: false
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// - name: sort
	//   in: query
	//   description: sort jobs by attribute. Supported values are "id". Default is "id"
	//   type: string
	// - name: order
	//   in: query
	//   description: sort order, either "asc" (ascending) or "desc" (descending). Default is "asc"
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/WorkflowJobsList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	repoID := ctx.Repo.Repository.ID

	shared.ListJobs(ctx, 0, repoID, 0, nil)
}

// ListWorkflowRuns Lists all runs for a repository run.
func (Action) ListWorkflowRuns(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs repository getWorkflowRuns
	// ---
	// summary: Lists all runs for a repository run
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
	// - name: event
	//   in: query
	//   description: workflow event name
	//   type: string
	//   required: false
	// - name: branch
	//   in: query
	//   description: workflow branch
	//   type: string
	//   required: false
	// - name: status
	//   in: query
	//   description: workflow status (requested, pending, queued, in_progress, failure, success, skipped)
	//   type: string
	//   required: false
	// - name: actor
	//   in: query
	//   description: triggered by user
	//   type: string
	//   required: false
	// - name: head_sha
	//   in: query
	//   description: triggering sha of the workflow run
	//   type: string
	//   required: false
	// - name: exclude_pull_requests
	//   in: query
	//   description: if true, the `pull_requests` field on each returned run is emptied
	//   type: boolean
	//   required: false
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
	//     "$ref": "#/responses/WorkflowRunsList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repoID := ctx.Repo.Repository.ID

	shared.ListRuns(ctx, 0, repoID, "")
}

var _ shared.API = new(Action)

// Action implements shared.API
type Action struct{}

// NewAction creates a new Action service
func NewAction() shared.API {
	return Action{}
}

// ListActionTasks list all the actions of a repository
func ListActionTasks(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/tasks repository ListActionTasks
	// ---
	// summary: List a repository's action tasks
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
	//   description: page size of results, default maximum page size is 50
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/TasksList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/conflict"
	//   "422":
	//     "$ref": "#/responses/validationError"
	listOptions := utils.GetListOptions(ctx)

	tasks, total, err := db.FindAndCount[actions_model.ActionTask](ctx, &actions_model.FindTaskOptions{
		ListOptions: listOptions,
		RepoID:      ctx.Repo.Repository.ID,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	res := new(api.ActionTaskResponse)
	res.TotalCount = total

	taskList := actions_model.TaskList(tasks)
	if err := taskList.LoadAttributes(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	res.Entries = make([]*api.ActionTask, len(tasks))
	for i := range tasks {
		convertedTask, err := convert.ToActionTask(ctx, tasks[i])
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		res.Entries[i] = convertedTask
	}

	ctx.SetLinkHeader(total, listOptions.PageSize)
	ctx.SetTotalCountHeader(total) // Duplicates api response field, but it's better to set it for consistency
	ctx.JSON(http.StatusOK, &res)
}
