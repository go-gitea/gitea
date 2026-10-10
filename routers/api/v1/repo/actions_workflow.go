// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"gitea.dev/actionslib/pkg/model"
	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/modules/optional"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/shared"
	actions_service "gitea.dev/services/actions"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

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

	workflows, err := convert.ListActionWorkflows(ctx, ctx.Repo.GitRepo, ctx.Repo.Repository)
	if err != nil {
		ctx.APIErrorInternal(err)
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
	workflow, err := convert.GetActionWorkflow(ctx, ctx.Repo.GitRepo, ctx.Repo.Repository, workflowID)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(http.StatusNotFound, err.Error())
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.JSON(http.StatusOK, workflow)
}

func ActionsListWorkflowRuns(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/workflows/{workflow_id}/runs repository ActionsListWorkflowRuns
	// ---
	// summary: List runs for a workflow
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
	//   description: id of the workflow, must be the workflow file name (e.g. `build.yml`)
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
	// - name: scoped_workflow_source_repo_id
	//   description: For a scoped workflow, the ID of the source repository providing it; omit or 0 for a repo-level workflow.
	//   in: query
	//   type: integer
	//   format: int64
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
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	workflowID := ctx.PathParam("workflow_id")
	scopedWorkflowSourceRepoID := ctx.FormInt64("scoped_workflow_source_repo_id")
	// Existing runs prove the workflow is/was valid and cover historical workflows whose file was later removed.
	// Repo-level never-run workflows fall back to a git lookup; scoped workflows are selected by source repo ID and may return an empty run list.
	runExists, err := db.Exist[actions_model.ActionRun](ctx, actions_model.FindRunOptions{
		RepoID:         ctx.Repo.Repository.ID,
		WorkflowID:     workflowID,
		WorkflowRepoID: scopedWorkflowSourceRepoID,
		IsScopedRun:    optional.Some(scopedWorkflowSourceRepoID > 0),
	}.ToConds())
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if !runExists {
		if scopedWorkflowSourceRepoID == 0 {
			if _, err := convert.GetActionWorkflow(ctx, ctx.Repo.GitRepo, ctx.Repo.Repository, workflowID); err != nil {
				ctx.APIErrorAuto(err)
				return
			}
		}
	}

	shared.ListRuns(ctx, 0, ctx.Repo.Repository.ID, workflowID)
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
			ctx.APIError(http.StatusNotFound, err.Error())
		} else {
			ctx.APIErrorInternal(err)
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
	// - name: return_run_details
	//   description: Whether the response should include the workflow run ID and URLs.
	//   in: query
	//   type: boolean
	// - name: scoped_workflow_source_repo_id
	//   description: For a scoped workflow, the ID of the source repository providing it; omit or 0 for a repo-level workflow.
	//   in: query
	//   type: integer
	//   format: int64
	// responses:
	//   "200":
	//     "$ref": "#/responses/RunDetails"
	//   "204":
	//     description: No Content, if return_run_details is missing or false
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	workflowID := ctx.PathParam("workflow_id")
	opt := web.GetForm[*api.CreateActionWorkflowDispatch](ctx)
	if opt.Ref == "" {
		ctx.APIError(http.StatusUnprocessableEntity, "ref is required parameter")
		return
	}

	// a non-zero scoped_workflow_source_repo_id dispatches a scoped workflow from that source repo; 0/absent is repo-level.
	scopedWorkflowSourceRepoID := ctx.FormInt64("scoped_workflow_source_repo_id")
	runID, err := actions_service.DispatchActionWorkflow(ctx, ctx.Doer, ctx.Repo.Repository, ctx.Repo.GitRepo, workflowID, opt.Ref, scopedWorkflowSourceRepoID, func(workflowDispatch *model.WorkflowDispatch, inputs map[string]any) error {
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
			ctx.APIError(http.StatusNotFound, err.Error())
		} else if errors.Is(err, util.ErrPermissionDenied) {
			ctx.APIError(http.StatusForbidden, err.Error())
		} else if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusUnprocessableEntity, err.Error())
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if !ctx.FormBool("return_run_details") {
		ctx.Status(http.StatusNoContent)
		return
	}

	ctx.JSON(http.StatusOK, &api.RunDetails{
		WorkflowRunID: runID,
		HTMLURL:       fmt.Sprintf("%s/actions/runs/%d", ctx.Repo.Repository.HTMLURL(ctx), runID),
		RunURL:        fmt.Sprintf("%s/actions/runs/%d", ctx.Repo.Repository.APIURL(), runID),
	})
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
			ctx.APIError(http.StatusNotFound, err.Error())
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}
