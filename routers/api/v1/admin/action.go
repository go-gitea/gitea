// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"errors"
	"net/http"

	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/shared"
	"code.gitea.io/gitea/services/context"
	secret_service "code.gitea.io/gitea/services/secrets"
)

// ListWorkflowJobs Lists all jobs
func ListWorkflowJobs(ctx *context.APIContext) {
	// swagger:operation GET /admin/actions/jobs admin listAdminWorkflowJobs
	// ---
	// summary: Lists all jobs
	// produces:
	// - application/json
	// parameters:
	// - name: status
	//   in: query
	//   description: workflow status (pending, queued, in_progress, failure, success, skipped)
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
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	shared.ListJobs(ctx, 0, 0, 0)
}

// ListWorkflowRuns Lists all runs
func ListWorkflowRuns(ctx *context.APIContext) {
	// swagger:operation GET /admin/actions/runs admin listAdminWorkflowRuns
	// ---
	// summary: Lists all runs
	// produces:
	// - application/json
	// parameters:
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
	//   description: workflow status (pending, queued, in_progress, failure, success, skipped)
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
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	shared.ListRuns(ctx, 0, 0)
}

// CreateOrUpdateSecret create or update one secret in instance scope
func CreateOrUpdateSecret(ctx *context.APIContext) {
	// swagger:operation PUT /admin/actions/secrets/{secretname} admin updateAdminSecret
	// ---
	// summary: Create or Update a secret value in instance scope
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	opt := web.GetForm(ctx).(*api.CreateOrUpdateSecretOption)

	_, created, err := secret_service.CreateOrUpdateSecret(ctx, 0, 0, ctx.PathParam("secretname"), opt.Data, opt.Description)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusBadRequest, err)
		} else if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(http.StatusNotFound, err)
		} else {
			ctx.APIError(http.StatusInternalServerError, err)
		}
		return
	}

	if created {
		ctx.Status(http.StatusCreated)
	} else {
		ctx.Status(http.StatusNoContent)
	}
}

// DeleteSecret delete one secret in instance scope
func DeleteSecret(ctx *context.APIContext) {
	// swagger:operation DELETE /admin/actions/secrets/{secretname} admin deleteAdminSecret
	// ---
	// summary: Delete a secret in instance scope
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: secretname
	//   in: path
	//   description: name of the secret
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: secret deleted
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	err := secret_service.DeleteSecretByName(ctx, 0, 0, ctx.PathParam("secretname"))
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusBadRequest, err)
		} else if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(http.StatusNotFound, err)
		} else {
			ctx.APIError(http.StatusInternalServerError, err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}
