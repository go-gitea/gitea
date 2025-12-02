// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// StartIssueStopwatch creates a stopwatch for the given issue.
func StartIssueStopwatch(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/stopwatch/start issue issueStartStopWatch
	// ---
	// summary: Start stopwatch on an issue.
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
	// - name: index
	//   in: path
	//   description: index of the issue to create the stopwatch on
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     description: Not repo writer, user does not have rights to toggle stopwatch
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     description: Cannot start a stopwatch again if it already exists

	issue := prepareIssueForStopwatch(ctx)
	if ctx.Written() {
		return
	}

	if ok, err := issues_model.CreateIssueStopwatch(ctx, ctx.Doer, issue); err != nil {
		ctx.APIErrorInternal(err)
		return
	} else if !ok {
		ctx.APIError(http.StatusConflict, "cannot start a stopwatch again if it already exists")
		return
	}

	ctx.Status(http.StatusCreated)
}

// StopIssueStopwatch stops a stopwatch for the given issue.
func StopIssueStopwatch(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/stopwatch/stop issue issueStopStopWatch
	// ---
	// summary: Stop an issue's existing stopwatch.
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
	// - name: index
	//   in: path
	//   description: index of the issue to stop the stopwatch on
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     description: Not repo writer, user does not have rights to toggle stopwatch
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     description:  Cannot stop a non-existent stopwatch

	issue := prepareIssueForStopwatch(ctx)
	if ctx.Written() {
		return
	}

	if ok, err := issues_model.FinishIssueStopwatch(ctx, ctx.Doer, issue); err != nil {
		ctx.APIErrorInternal(err)
		return
	} else if !ok {
		ctx.APIError(http.StatusConflict, "cannot stop a non-existent stopwatch")
		return
	}
	ctx.Status(http.StatusCreated)
}

// DeleteIssueStopwatch delete a specific stopwatch
func DeleteIssueStopwatch(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/{index}/stopwatch/delete issue issueDeleteStopWatch
	// ---
	// summary: Delete an issue's existing stopwatch.
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
	// - name: index
	//   in: path
	//   description: index of the issue to stop the stopwatch on
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     description: Not repo writer, user does not have rights to toggle stopwatch
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     description:  Cannot cancel a non-existent stopwatch

	issue := prepareIssueForStopwatch(ctx)
	if ctx.Written() {
		return
	}

	if ok, err := issues_model.CancelStopwatch(ctx, ctx.Doer, issue); err != nil {
		ctx.APIErrorInternal(err)
		return
	} else if !ok {
		ctx.APIError(http.StatusConflict, "cannot cancel a non-existent stopwatch")
		return
	}

	ctx.Status(http.StatusNoContent)
}

func prepareIssueForStopwatch(ctx *context.APIContext) *issues_model.Issue {
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return nil
	}

	if !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		ctx.Status(http.StatusForbidden)
		return nil
	}

	if !ctx.Repo.CanUseTimetracker(ctx, issue, ctx.Doer) {
		ctx.Status(http.StatusForbidden)
		return nil
	}
	return issue
}

// GetStopwatches get all stopwatches
func GetStopwatches(ctx *context.APIContext) {
	// swagger:operation GET /user/stopwatches user userGetStopWatches
	// ---
	// summary: Get list of all existing stopwatches
	// parameters:
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/StopWatchList"

	sws, err := issues_model.GetUserStopwatches(ctx, ctx.Doer.ID, utils.GetListOptions(ctx))
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	count, err := issues_model.CountUserStopwatches(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiSWs, err := convert.ToStopWatches(ctx, sws)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, apiSWs)
}
