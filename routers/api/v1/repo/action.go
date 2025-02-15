// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	go_context "context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	secret_model "code.gitea.io/gitea/models/secret"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/shared"
	"code.gitea.io/gitea/routers/api/v1/utils"
	actions_service "code.gitea.io/gitea/services/actions"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	secret_service "code.gitea.io/gitea/services/secrets"

	"github.com/nektos/act/pkg/model"
)

// ListActionsSecrets list an repo's actions secrets
func (Action) ListActionsSecrets(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/secrets repository repoListActionsSecrets
	// ---
	// summary: List an repo's actions secrets
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

	opts := &secret_model.FindSecretsOptions{
		RepoID:      repo.ID,
		ListOptions: utils.GetListOptions(ctx),
	}

	secrets, count, err := db.FindAndCount[secret_model.Secret](ctx, opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	apiSecrets := make([]*api.Secret, len(secrets))
	for k, v := range secrets {
		apiSecrets[k] = &api.Secret{
			Name:    v.Name,
			Created: v.CreatedUnix.AsTime(),
		}
	}

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

	opt := web.GetForm(ctx).(*api.CreateOrUpdateSecretOption)

	_, created, err := secret_service.CreateOrUpdateSecret(ctx, 0, repo.ID, ctx.PathParam("secretname"), opt.Data)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusBadRequest, "CreateOrUpdateSecret", err)
		} else if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "CreateOrUpdateSecret", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "CreateOrUpdateSecret", err)
		}
		return
	}

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
	//     description: delete one secret of the organization
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repo := ctx.Repo.Repository

	err := secret_service.DeleteSecretByName(ctx, 0, repo.ID, ctx.PathParam("secretname"))
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusBadRequest, "DeleteSecret", err)
		} else if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "DeleteSecret", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteSecret", err)
		}
		return
	}

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
	//   description: name of the owner
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
			ctx.Error(http.StatusNotFound, "GetVariable", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetVariable", err)
		}
		return
	}

	variable := &api.ActionVariable{
		OwnerID: v.OwnerID,
		RepoID:  v.RepoID,
		Name:    v.Name,
		Data:    v.Data,
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
	//   description: name of the owner
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
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusBadRequest, "DeleteVariableByName", err)
		} else if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "DeleteVariableByName", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteVariableByName", err)
		}
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
	//   description: name of the owner
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
	//   "204":
	//     description: response when creating a repo-level variable
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opt := web.GetForm(ctx).(*api.CreateVariableOption)

	repoID := ctx.Repo.Repository.ID
	variableName := ctx.PathParam("variablename")

	v, err := actions_service.GetVariable(ctx, actions_model.FindVariablesOpts{
		RepoID: repoID,
		Name:   variableName,
	})
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		ctx.Error(http.StatusInternalServerError, "GetVariable", err)
		return
	}
	if v != nil && v.ID > 0 {
		ctx.Error(http.StatusConflict, "VariableNameAlreadyExists", util.NewAlreadyExistErrorf("variable name %s already exists", variableName))
		return
	}

	if _, err := actions_service.CreateVariable(ctx, 0, repoID, variableName, opt.Value); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusBadRequest, "CreateVariable", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "CreateVariable", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
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
	//   description: name of the owner
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

	opt := web.GetForm(ctx).(*api.UpdateVariableOption)

	v, err := actions_service.GetVariable(ctx, actions_model.FindVariablesOpts{
		RepoID: ctx.Repo.Repository.ID,
		Name:   ctx.PathParam("variablename"),
	})
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "GetVariable", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetVariable", err)
		}
		return
	}

	if opt.Name == "" {
		opt.Name = ctx.PathParam("variablename")
	}

	v.Name = opt.Name
	v.Data = opt.Value

	if _, err := actions_service.UpdateVariableNameData(ctx, v); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusBadRequest, "UpdateVariable", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "UpdateVariable", err)
		}
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
	//   description: name of the owner
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

	vars, count, err := db.FindAndCount[actions_model.ActionVariable](ctx, &actions_model.FindVariablesOpts{
		RepoID:      ctx.Repo.Repository.ID,
		ListOptions: utils.GetListOptions(ctx),
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindVariables", err)
		return
	}

	variables := make([]*api.ActionVariable, len(vars))
	for i, v := range vars {
		variables[i] = &api.ActionVariable{
			OwnerID: v.OwnerID,
			RepoID:  v.RepoID,
			Name:    v.Name,
		}
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, variables)
}

// GetRegistrationToken returns the token to register repo runners
func (Action) GetRegistrationToken(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runners/registration-token repository repoGetRunnerRegistrationToken
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

var _ actions_service.API = new(Action)

// Action implements actions_service.API
type Action struct{}

// NewAction creates a new Action service
func NewAction() actions_service.API {
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

	tasks, total, err := db.FindAndCount[actions_model.ActionTask](ctx, &actions_model.FindTaskOptions{
		ListOptions: utils.GetListOptions(ctx),
		RepoID:      ctx.Repo.Repository.ID,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ListActionTasks", err)
		return
	}

	res := new(api.ActionTaskResponse)
	res.TotalCount = total

	res.Entries = make([]*api.ActionTask, len(tasks))
	for i := range tasks {
		convertedTask, err := convert.ToActionTask(ctx, tasks[i])
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "ToActionTask", err)
			return
		}
		res.Entries[i] = convertedTask
	}

	ctx.JSON(http.StatusOK, &res)
}

func ActionsListRepositoryWorkflows(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/workflows repository ActionsListRepositoryWorkflows
	// ---
	// summary: List repository workflows
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
	//     "$ref": "#/responses/ActionWorkflowList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "500":
	//     "$ref": "#/responses/error"

	workflows, err := actions_service.ListActionWorkflows(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ListActionWorkflows", err)
		return
	}

	ctx.JSON(http.StatusOK, &api.ActionWorkflowResponse{Workflows: workflows, TotalCount: int64(len(workflows))})
}

func ActionsGetWorkflow(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/workflows/{workflow_id} repository ActionsGetWorkflow
	// ---
	// summary: Get a workflow
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
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActionWorkflow"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "500":
	//     "$ref": "#/responses/error"

	workflowID := ctx.PathParam("workflow_id")
	workflow, err := actions_service.GetActionWorkflow(ctx, workflowID)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "GetActionWorkflow", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetActionWorkflow", err)
		}
		return
	}

	ctx.JSON(http.StatusOK, workflow)
}

func ActionsDisableWorkflow(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/actions/workflows/{workflow_id}/disable repository ActionsDisableWorkflow
	// ---
	// summary: Disable a workflow
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
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: No Content
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	workflowID := ctx.PathParam("workflow_id")
	err := actions_service.EnableOrDisableWorkflow(ctx, workflowID, false)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "DisableActionWorkflow", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "DisableActionWorkflow", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

func ActionsDispatchWorkflow(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/workflows/{workflow_id}/dispatches repository ActionsDispatchWorkflow
	// ---
	// summary: Create a workflow dispatch event
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
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateActionWorkflowDispatch"
	// responses:
	//   "204":
	//     description: No Content
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	workflowID := ctx.PathParam("workflow_id")
	opt := web.GetForm(ctx).(*api.CreateActionWorkflowDispatch)
	if opt.Ref == "" {
		ctx.Error(http.StatusUnprocessableEntity, "MissingWorkflowParameter", util.NewInvalidArgumentErrorf("ref is required parameter"))
		return
	}

	err := actions_service.DispatchActionWorkflow(ctx, ctx.Doer, ctx.Repo.Repository, ctx.Repo.GitRepo, workflowID, opt.Ref, func(workflowDispatch *model.WorkflowDispatch, inputs map[string]any) error {
		if strings.Contains(ctx.Req.Header.Get("Content-Type"), "form-urlencoded") {
			// The chi framework's "Binding" doesn't support to bind the form map values into a map[string]string
			// So we have to manually read the `inputs[key]` from the form
			for name, config := range workflowDispatch.Inputs {
				value := ctx.FormString("inputs["+name+"]", config.Default)
				inputs[name] = value
			}
		} else {
			for name, config := range workflowDispatch.Inputs {
				value, ok := opt.Inputs[name]
				if ok {
					inputs[name] = value
				} else {
					inputs[name] = config.Default
				}
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "DispatchActionWorkflow", err)
		} else if errors.Is(err, util.ErrPermissionDenied) {
			ctx.Error(http.StatusForbidden, "DispatchActionWorkflow", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "DispatchActionWorkflow", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

func ActionsEnableWorkflow(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/actions/workflows/{workflow_id}/enable repository ActionsEnableWorkflow
	// ---
	// summary: Enable a workflow
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
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: No Content
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

	workflowID := ctx.PathParam("workflow_id")
	err := actions_service.EnableOrDisableWorkflow(ctx, workflowID, true)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "EnableActionWorkflow", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "EnableActionWorkflow", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// GetArtifacts Lists all artifacts for a repository.
func GetArtifactsOfRun(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs/{run}/artifacts repository getArtifactsOfRun
	// ---
	// summary: Lists all artifacts for a repository run
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: name of the owner
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: run
	//   in: path
	//   description: runid of the workflow run
	//   type: integer
	//   required: true
	// - name: name
	//   in: query
	//   description: name of the artifact
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/responses/ArtifactsList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repoID := ctx.Repo.Repository.ID
	artifactName := ctx.Req.URL.Query().Get("name")

	runID := ctx.PathParamInt64("run")

	artifacts, total, err := db.FindAndCount[actions_model.ActionArtifact](ctx, actions_model.FindArtifactsOptions{
		RepoID:               repoID,
		RunID:                runID,
		ArtifactName:         artifactName,
		FinalizedArtifactsV4: true,
		ListOptions:          utils.GetListOptions(ctx),
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error(), err)
		return
	}

	res := new(api.ActionArtifactsResponse)
	res.TotalCount = total

	res.Entries = make([]*api.ActionArtifact, len(artifacts))
	for i := range artifacts {
		convertedArtifact, err := convert.ToActionArtifact(ctx.Repo.Repository, artifacts[i])
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "ToActionArtifact", err)
			return
		}
		res.Entries[i] = convertedArtifact
	}

	ctx.JSON(http.StatusOK, &res)
}

// GetArtifacts Lists all artifacts for a repository.
func GetArtifacts(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/artifacts repository getArtifacts
	// ---
	// summary: Lists all artifacts for a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: name of the owner
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: name
	//   in: query
	//   description: name of the artifact
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/responses/ArtifactsList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repoID := ctx.Repo.Repository.ID
	artifactName := ctx.Req.URL.Query().Get("name")

	artifacts, total, err := db.FindAndCount[actions_model.ActionArtifact](ctx, actions_model.FindArtifactsOptions{
		RepoID:               repoID,
		ArtifactName:         artifactName,
		FinalizedArtifactsV4: true,
		ListOptions:          utils.GetListOptions(ctx),
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error(), err)
		return
	}

	res := new(api.ActionArtifactsResponse)
	res.TotalCount = total

	res.Entries = make([]*api.ActionArtifact, len(artifacts))
	for i := range artifacts {
		convertedArtifact, err := convert.ToActionArtifact(ctx.Repo.Repository, artifacts[i])
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "ToActionArtifact", err)
			return
		}
		res.Entries[i] = convertedArtifact
	}

	ctx.JSON(http.StatusOK, &res)
}

// GetArtifact Gets a specific artifact for a workflow run.
func GetArtifact(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/artifacts/{artifact_id} repository getArtifact
	// ---
	// summary: Gets a specific artifact for a workflow run
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: name of the owner
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: artifact_id
	//   in: path
	//   description: id of the artifact
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Artifact"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	art := getArtifactByPathParam(ctx, ctx.Repo.Repository)
	if ctx.Written() {
		return
	}

	if actions.IsArtifactV4(art) {
		convertedArtifact, err := convert.ToActionArtifact(ctx.Repo.Repository, art)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "ToActionArtifact", err)
			return
		}
		ctx.JSON(http.StatusOK, convertedArtifact)
		return
	}
	// v3 not supported due to not having one unique id
	ctx.Error(http.StatusNotFound, "GetArtifact", "Artifact not found")
}

// DeleteArtifact Deletes a specific artifact for a workflow run.
func DeleteArtifact(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/actions/artifacts/{artifact_id} repository deleteArtifact
	// ---
	// summary: Deletes a specific artifact for a workflow run
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: name of the owner
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: artifact_id
	//   in: path
	//   description: id of the artifact
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: "No Content"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	art := getArtifactByPathParam(ctx, ctx.Repo.Repository)
	if ctx.Written() {
		return
	}

	if actions.IsArtifactV4(art) {
		if err := actions_model.SetArtifactNeedDelete(ctx, art.RunID, art.ArtifactName); err != nil {
			ctx.Error(http.StatusInternalServerError, "DeleteArtifact", err)
			return
		}
		ctx.Status(http.StatusNoContent)
		return
	}
	// v3 not supported due to not having one unique id
	ctx.Error(http.StatusNotFound, "DeleteArtifact", "Artifact not found")
}

func buildSignature(endp, expires string, artifactID int64) []byte {
	mac := hmac.New(sha256.New, setting.GetGeneralTokenSigningSecret())
	mac.Write([]byte(endp))
	mac.Write([]byte(expires))
	mac.Write([]byte(fmt.Sprint(artifactID)))
	return mac.Sum(nil)
}

func buildDownloadRawEndpoint(repo *repo_model.Repository, artifactID int64) string {
	return fmt.Sprintf("api/v1/repos/%s/%s//actions/artifacts/%d/zip/raw", url.PathEscape(repo.OwnerName), url.PathEscape(repo.Name), artifactID)
}

func buildSigURL(ctx go_context.Context, endPoint string, artifactID int64) string {
	// endPoint is a path like "api/v1/repos/owner/repo/actions/artifacts/1/zip/raw"
	expires := time.Now().Add(60 * time.Minute).Format("2006-01-02 15:04:05.999999999 -0700 MST")
	uploadURL := httplib.GuessCurrentAppURL(ctx) + endPoint + "?sig=" + base64.URLEncoding.EncodeToString(buildSignature(endPoint, expires, artifactID)) + "&expires=" + url.QueryEscape(expires)
	return uploadURL
}

// DownloadArtifact Downloads a specific artifact for a workflow run redirects to blob url.
func DownloadArtifact(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/artifacts/{artifact_id}/zip repository downloadArtifact
	// ---
	// summary: Downloads a specific artifact for a workflow run redirects to blob url
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: name of the owner
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: artifact_id
	//   in: path
	//   description: id of the artifact
	//   type: string
	//   required: true
	// responses:
	//   "302":
	//     description: redirect to the blob download
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	art := getArtifactByPathParam(ctx, ctx.Repo.Repository)
	if ctx.Written() {
		return
	}

	// if artifacts status is not uploaded-confirmed, treat it as not found
	if art.Status == actions_model.ArtifactStatusExpired {
		ctx.Error(http.StatusNotFound, "DownloadArtifact", "Artifact has expired")
		return
	}
	ctx.Resp.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip; filename*=UTF-8''%s.zip", url.PathEscape(art.ArtifactName), art.ArtifactName))

	if actions.IsArtifactV4(art) {
		ok, err := actions.DownloadArtifactV4ServeDirectOnly(ctx.Base, art)
		if ok {
			return
		}
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "DownloadArtifactV4ServeDirectOnly", err)
			return
		}

		redirectURL := buildSigURL(ctx, buildDownloadRawEndpoint(ctx.Repo.Repository, art.ID), art.ID)
		ctx.Redirect(redirectURL, http.StatusFound)
		return
	}
	// v3 not supported due to not having one unique id
	ctx.Error(http.StatusNotFound, "DownloadArtifact", "Artifact not found")
}

// DownloadArtifactRaw Downloads a specific artifact for a workflow run directly.
func DownloadArtifactRaw(ctx *context.APIContext) {
	// TODO: if it needs to skip "repoAssignment" middleware, it could query the repo from path params: ctx.PathParam("username"), ctx.PathParam("reponame")
	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, ctx.PathParam("username"), ctx.PathParam("reponame"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.NotFound()
		} else {
			ctx.InternalServerError(err)
		}
		return
	}
	art := getArtifactByPathParam(ctx, repo)
	if ctx.Written() {
		return
	}

	sigStr := ctx.Req.URL.Query().Get("sig")
	expires := ctx.Req.URL.Query().Get("expires")
	sigBytes, _ := base64.URLEncoding.DecodeString(sigStr)

	expectedSig := buildSignature(buildDownloadRawEndpoint(ctx.Repo.Repository, art.ID), expires, art.ID)
	if !hmac.Equal(sigBytes, expectedSig) {
		ctx.Error(http.StatusUnauthorized, "DownloadArtifactRaw", "Error unauthorized")
		return
	}
	t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", expires)
	if err != nil || t.Before(time.Now()) {
		ctx.Error(http.StatusUnauthorized, "DownloadArtifactRaw", "Error link expired")
		return
	}

	// if artifacts status is not uploaded-confirmed, treat it as not found
	if art.Status == actions_model.ArtifactStatusExpired {
		ctx.Error(http.StatusNotFound, "DownloadArtifactRaw", "Artifact has expired")
		return
	}
	ctx.Resp.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip; filename*=UTF-8''%s.zip", url.PathEscape(art.ArtifactName), art.ArtifactName))

	if actions.IsArtifactV4(art) {
		err := actions.DownloadArtifactV4(ctx.Base, art)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "DownloadArtifactV4", err)
			return
		}
		return
	}
	// v3 not supported due to not having one unique id
	ctx.Error(http.StatusNotFound, "DownloadArtifactRaw", "artifact not found")
}

// Try to get the artifact by ID and check access
func getArtifactByPathParam(ctx *context.APIContext, repo *repo_model.Repository) *actions_model.ActionArtifact {
	artifactID := ctx.PathParamInt64("artifact_id")

	art, ok, err := db.GetByID[actions_model.ActionArtifact](ctx, artifactID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "getArtifactByPathParam", err)
		return nil
	}
	// if artifacts status is not uploaded-confirmed, treat it as not found
	// FIXME: is the OwnerID check right? What if a repo is transferred to a new owner?
	if !ok ||
		(art.RepoID != repo.ID || art.OwnerID != repo.OwnerID) ||
		art.Status != actions_model.ArtifactStatusUploadConfirmed && art.Status != actions_model.ArtifactStatusExpired {
		ctx.Error(http.StatusNotFound, "getArtifactByPathParam", "artifact not found")
		return nil
	}
	return art
}
