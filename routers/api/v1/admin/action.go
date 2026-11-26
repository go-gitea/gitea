// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	actions_model "gitea.dev/models/actions"
	"gitea.dev/routers/api/v1/shared"
	"gitea.dev/services/context"
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
	//   default:
	//     "$ref": "#/responses/error"

	shared.ListJobs(ctx, 0, 0, 0, nil)
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

	shared.ListRuns(ctx, 0, 0, "")
}

func ListVariables(ctx *context.APIContext) {
	// swagger:operation GET /admin/actions/variables admin getAdminVariablesList
	// ---
	// summary: Get the instance-level variables list
	// produces:
	// - application/json
	// parameters:
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
	//   default:
	//     "$ref": "#/responses/error"
	shared.ListVariables(ctx, actions_model.BelongingScopeGlobal())
}

func GetVariable(ctx *context.APIContext) {
	// swagger:operation GET /admin/actions/variables/{variablename} admin getAdminVariable
	// ---
	// summary: Get an instance-level variable
	// produces:
	// - application/json
	// parameters:
	// - name: variablename
	//   in: path
	//   description: name of the variable
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActionVariable"
	//   default:
	//     "$ref": "#/responses/error"
	shared.GetVariable(ctx, actions_model.BelongingScopeGlobal())
}

func CreateVariable(ctx *context.APIContext) {
	// swagger:operation POST /admin/actions/variables/{variablename} admin createAdminVariable
	// ---
	// summary: Create an instance-level variable
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
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
	//     description: successfully created the instance-level variable
	//   default:
	//     "$ref": "#/responses/error"
	shared.CreateVariable(ctx, actions_model.BelongingScopeGlobal())
}

func UpdateVariable(ctx *context.APIContext) {
	// swagger:operation PUT /admin/actions/variables/{variablename} admin updateAdminVariable
	// ---
	// summary: Update an instance-level variable
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
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
	//     description: response when updating an instance-level variable
	//   default:
	//     "$ref": "#/responses/error"
	shared.UpdateVariable(ctx, actions_model.BelongingScopeGlobal())
}

func DeleteVariable(ctx *context.APIContext) {
	// swagger:operation DELETE /admin/actions/variables/{variablename} admin deleteAdminVariable
	// ---
	// summary: Delete an instance-level variable
	// produces:
	// - application/json
	// parameters:
	// - name: variablename
	//   in: path
	//   description: name of the variable
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: response when deleting an instance-level variable
	//   default:
	//     "$ref": "#/responses/error"
	shared.DeleteVariable(ctx, actions_model.BelongingScopeGlobal())
}
