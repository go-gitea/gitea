// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"code.gitea.io/gitea/routers/api/v1/shared"
	"code.gitea.io/gitea/services/context"
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
