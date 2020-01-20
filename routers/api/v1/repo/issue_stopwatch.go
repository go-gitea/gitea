// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
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

	if err := models.CreateOrStopIssueStopwatch(ctx.User, issue); err != nil {
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

	if err := models.CreateOrStopIssueStopwatch(ctx.User, issue); err != nil {
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

	if err := models.CancelStopwatch(ctx.User, issue); err != nil {
		ctx.Error(http.StatusInternalServerError, "CancelStopwatch", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func prepareIssueStopwatch(ctx *context.APIContext, shouldExist bool) (*models.Issue, error) {
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}

		return nil, err
	}

	if !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		ctx.Status(http.StatusForbidden)
		return nil, err
	}

	if !ctx.Repo.CanUseTimetracker(issue, ctx.User) {
		ctx.Status(http.StatusForbidden)
		return nil, err
	}

	if models.StopwatchExists(ctx.User.ID, issue.ID) != shouldExist {
		if shouldExist {
			ctx.Error(http.StatusConflict, "StopwatchExists", "cannot stop/cancel a non existent stopwatch")
		} else {
			ctx.Error(http.StatusConflict, "StopwatchExists", "cannot start a stopwatch again if it already exists")
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
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/StopWatchList"

	sws, err := models.GetUserStopwatches(ctx.User.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUserStopwatches", err)
		return
	}

	apiSWs, err := sws.APIFormat()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "APIFormat", err)
		return
	}

	ctx.JSON(http.StatusOK, apiSWs)
}
