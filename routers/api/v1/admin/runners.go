// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"code.gitea.io/gitea/routers/api/v1/shared"
	"code.gitea.io/gitea/services/context"
)

// https://docs.github.com/en/rest/actions/self-hosted-runners?apiVersion=2022-11-28#create-a-registration-token-for-an-organization

// GetRegistrationToken returns the token to register global runners
func GetRegistrationToken(ctx *context.APIContext) {
	// swagger:operation GET /admin/runners/registration-token admin adminGetRunnerRegistrationToken
	// ---
	// summary: Get an global actions runner registration token
	// produces:
	// - application/json
	// parameters:
	// responses:
	//   "200":
	//     "$ref": "#/responses/RegistrationToken"

	shared.GetRegistrationToken(ctx, 0, 0)
}

// CreateRegistrationToken returns the token to register global runners
func CreateRegistrationToken(ctx *context.APIContext) {
	// swagger:operation POST /admin/actions/runners/registration-token admin adminCreateRunnerRegistrationToken
	// ---
	// summary: Get an global actions runner registration token
	// produces:
	// - application/json
	// parameters:
	// responses:
	//   "200":
	//     "$ref": "#/responses/RegistrationToken"

	shared.GetRegistrationToken(ctx, 0, 0)
}

// ListRunners get all runners
func ListRunners(ctx *context.APIContext) {
	// swagger:operation GET /admin/actions/runners admin getAdminRunners
	// ---
	// summary: Get all runners
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/definitions/ActionRunnersResponse"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	shared.ListRunners(ctx, 0, 0)
}

// GetRunner get an global runner
func GetRunner(ctx *context.APIContext) {
	// swagger:operation GET /admin/actions/runners/{runner_id} admin getAdminRunner
	// ---
	// summary: Get an global runner
	// produces:
	// - application/json
	// parameters:
	// - name: runner_id
	//   in: path
	//   description: id of the runner
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/definitions/ActionRunner"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	shared.GetRunner(ctx, 0, 0, ctx.PathParamInt64("runner_id"))
}

// DeleteRunner delete an global runner
func DeleteRunner(ctx *context.APIContext) {
	// swagger:operation DELETE /admin/actions/runners/{runner_id} admin deleteAdminRunner
	// ---
	// summary: Delete an global runner
	// produces:
	// - application/json
	// parameters:
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
	shared.DeleteRunner(ctx, 0, 0, ctx.PathParamInt64("runner_id"))
}

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
