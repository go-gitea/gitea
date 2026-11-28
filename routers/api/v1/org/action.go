// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	secret_model "gitea.dev/models/secret"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/shared"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/audit"
	"gitea.dev/services/context"
	secret_service "gitea.dev/services/secrets"
)

// ListActionsSecrets list an organization's actions secrets
func (Action) ListActionsSecrets(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/actions/secrets organization orgListActionsSecrets
	// ---
	// summary: List an organization's actions secrets
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
	//   default:
	//     "$ref": "#/responses/error"

	opts := &secret_model.FindSecretsOptions{
		BelongingScope: actions_model.BelongingScopeOwner(ctx.Org.Organization.ID),
		ListOptions:    utils.GetListOptions(ctx),
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

	ctx.SetLinkHeader(count, opts.PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, apiSecrets)
}

// create or update one secret of the organization
func (Action) CreateOrUpdateSecret(ctx *context.APIContext) {
	// swagger:operation PUT /orgs/{org}/actions/secrets/{secretname} organization updateOrgSecret
	// ---
	// summary: Create or Update a secret value in an organization
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
	//   default:
	//     "$ref": "#/responses/error"

	opt := web.GetForm[*api.CreateOrUpdateSecretOption](ctx)

	s, created, err := secret_service.CreateOrUpdateSecret(ctx, actions_model.BelongingScopeOwner(ctx.Org.Organization.ID), ctx.PathParam("secretname"), opt.Data, opt.Description)
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	actions := audit.SecretUpdate
	if created {
		actions = audit.SecretAdd
	}
	audit.RecordScoped(ctx, ctx.Org.Organization.AsUser(), nil, actions, "secret", s.Name)

	if created {
		ctx.Status(http.StatusCreated)
	} else {
		ctx.Status(http.StatusNoContent)
	}
}

// DeleteSecret delete one secret of the organization
func (Action) DeleteSecret(ctx *context.APIContext) {
	// swagger:operation DELETE /orgs/{org}/actions/secrets/{secretname} organization deleteOrgSecret
	// ---
	// summary: Delete a secret in an organization
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
	// - name: secretname
	//   in: path
	//   description: name of the secret
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: delete one secret of the organization
	//   default:
	//     "$ref": "#/responses/error"

	s, err := secret_service.DeleteSecretByName(ctx, actions_model.BelongingScopeOwner(ctx.Org.Organization.ID), ctx.PathParam("secretname"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	audit.RecordScoped(ctx, ctx.Org.Organization.AsUser(), nil, audit.SecretRemove, "secret", s.Name)

	ctx.Status(http.StatusNoContent)
}

// https://docs.github.com/en/rest/actions/self-hosted-runners?apiVersion=2022-11-28#create-a-registration-token-for-an-organization
// CreateRegistrationToken returns the token to register org runners
func (Action) CreateRegistrationToken(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/actions/runners/registration-token organization orgCreateRunnerRegistrationToken
	// ---
	// summary: Get an organization's actions runner registration token
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/RegistrationToken"

	shared.GetRegistrationToken(ctx, ctx.Org.Organization.ID, 0)
}

// ListVariables list org-level variables
func (Action) ListVariables(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/actions/variables organization getOrgVariablesList
	// ---
	// summary: Get an org-level variables list
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
	//   default:
	//     "$ref": "#/responses/error"
	shared.ListVariables(ctx, actions_model.BelongingScopeOwner(ctx.Org.Organization.ID))
}

// GetVariable get an org-level variable
func (Action) GetVariable(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/actions/variables/{variablename} organization getOrgVariable
	// ---
	// summary: Get an org-level variable
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: variablename
	//   in: path
	//   description: name of the variable
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//		 "$ref": "#/responses/ActionVariable"
	//   default:
	//     "$ref": "#/responses/error"
	shared.GetVariable(ctx, actions_model.BelongingScopeOwner(ctx.Org.Organization.ID))
}

// DeleteVariable delete an org-level variable
func (Action) DeleteVariable(ctx *context.APIContext) {
	// swagger:operation DELETE /orgs/{org}/actions/variables/{variablename} organization deleteOrgVariable
	// ---
	// summary: Delete an org-level variable
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
	//   default:
	//     "$ref": "#/responses/error"
	shared.DeleteVariable(ctx, actions_model.BelongingScopeOwner(ctx.Org.Organization.ID))
}

// CreateVariable create an org-level variable
func (Action) CreateVariable(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/actions/variables/{variablename} organization createOrgVariable
	// ---
	// summary: Create an org-level variable
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
	//     description: successfully created the org-level variable
	//   default:
	//     "$ref": "#/responses/error"
	shared.CreateVariable(ctx, actions_model.BelongingScopeOwner(ctx.Org.Organization.ID))
}

// UpdateVariable update an org-level variable
func (Action) UpdateVariable(ctx *context.APIContext) {
	// swagger:operation PUT /orgs/{org}/actions/variables/{variablename} organization updateOrgVariable
	// ---
	// summary: Update an org-level variable
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
	//     description: response when updating an org-level variable
	//   "204":
	//     description: response when updating an org-level variable
	//   default:
	//     "$ref": "#/responses/error"
	shared.UpdateVariable(ctx, actions_model.BelongingScopeOwner(ctx.Org.Organization.ID))
}

// ListRunners get org-level runners
func (Action) ListRunners(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/actions/runners organization getOrgRunners
	// ---
	// summary: Get org-level runners
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
	//   default:
	//     "$ref": "#/responses/error"
	shared.ListRunners(ctx, ctx.Org.Organization.ID, 0)
}

// GetRunner get an org-level runner
func (Action) GetRunner(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/actions/runners/{runner_id} organization getOrgRunner
	// ---
	// summary: Get an org-level runner
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
	//   default:
	//     "$ref": "#/responses/error"
	shared.GetRunner(ctx, ctx.Org.Organization.ID, 0, ctx.PathParamInt64("runner_id"))
}

// DeleteRunner delete an org-level runner
func (Action) DeleteRunner(ctx *context.APIContext) {
	// swagger:operation DELETE /orgs/{org}/actions/runners/{runner_id} organization deleteOrgRunner
	// ---
	// summary: Delete an org-level runner
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
	//   default:
	//     "$ref": "#/responses/error"
	shared.DeleteRunner(ctx, ctx.Org.Organization.ID, 0, ctx.PathParamInt64("runner_id"))
}

// UpdateRunner update an org-level runner
func (Action) UpdateRunner(ctx *context.APIContext) {
	// swagger:operation PATCH /orgs/{org}/actions/runners/{runner_id} organization updateOrgRunner
	// ---
	// summary: Update an org-level runner
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
	//   default:
	//     "$ref": "#/responses/error"
	shared.UpdateRunner(ctx, ctx.Org.Organization.ID, 0, ctx.PathParamInt64("runner_id"))
}

func (Action) ListWorkflowJobs(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/actions/jobs organization getOrgWorkflowJobs
	// ---
	// summary: Get org-level workflow jobs
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/WorkflowJobsList"
	//   default:
	//     "$ref": "#/responses/error"
	shared.ListJobs(ctx, ctx.Org.Organization.ID, 0, 0, nil)
}

func (Action) ListWorkflowRuns(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/actions/runs organization getOrgWorkflowRuns
	// ---
	// summary: Get org-level workflow runs
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
	//   default:
	//     "$ref": "#/responses/error"
	shared.ListRuns(ctx, ctx.Org.Organization.ID, 0, "")
}

var _ shared.ActionsAPI = new(Action)

// Action implements shared.ActionsAPI
type Action struct{}

// NewAction creates a new Action service
func NewAction() shared.ActionsAPI {
	return Action{}
}
