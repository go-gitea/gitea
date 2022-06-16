// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"errors"
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/routers/api/v1/utils"
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

	issue, err := prepareIssueStopwatch(ctx, false)
	if err != nil {
		return
	}

	if err := issues_model.CreateIssueStopwatch(ctx, ctx.Doer, issue); err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateOrStopIssueStopwatch", err)
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
	//     description:  Cannot stop a non existent stopwatch

	issue, err := prepareIssueStopwatch(ctx, true)
	if err != nil {
		return
	}

	if err := issues_model.FinishIssueStopwatch(ctx, ctx.Doer, issue); err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateOrStopIssueStopwatch", err)
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
	//     description:  Cannot cancel a non existent stopwatch

	issue, err := prepareIssueStopwatch(ctx, true)
	if err != nil {
		return
	}

	if err := issues_model.CancelStopwatch(ctx.Doer, issue); err != nil {
		ctx.Error(http.StatusInternalServerError, "CancelStopwatch", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func prepareIssueStopwatch(ctx *context.APIContext, shouldExist bool) (*issues_model.Issue, error) {
	issue, err := issues_model.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}

		return nil, err
	}

	if !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		ctx.Status(http.StatusForbidden)
		return nil, errors.New("Unable to write to PRs")
	}

	if !ctx.Repo.CanUseTimetracker(issue, ctx.Doer) {
		ctx.Status(http.StatusForbidden)
		return nil, errors.New("Cannot use time tracker")
	}

	if issues_model.StopwatchExists(ctx.Doer.ID, issue.ID) != shouldExist {
		if shouldExist {
			ctx.Error(http.StatusConflict, "StopwatchExists", "cannot stop/cancel a non existent stopwatch")
			err = errors.New("cannot stop/cancel a non existent stopwatch")
		} else {
			ctx.Error(http.StatusConflict, "StopwatchExists", "cannot start a stopwatch again if it already exists")
			err = errors.New("cannot start a stopwatch again if it already exists")
		}
		return nil, err
	}

	return issue, nil
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

	sws, err := issues_model.GetUserStopwatches(ctx.Doer.ID, utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUserStopwatches", err)
		return
	}

	count, err := issues_model.CountUserStopwatches(ctx.Doer.ID)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	apiSWs, err := convert.ToStopWatches(sws)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "APIFormat", err)
		return
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, apiSWs)
}
