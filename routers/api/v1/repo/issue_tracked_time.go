// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
)

// ListTrackedTimes list all the tracked times of an issue
func ListTrackedTimes(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{index}/times issue issueTrackedTimes
	// ---
	// summary: List an issue's tracked times
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
	//   description: index of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/TrackedTimeList"
	if !ctx.Repo.Repository.IsTimetrackerEnabled() {
		ctx.NotFound("Timetracker is disabled")
		return
	}
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(500, "GetIssueByIndex", err)
		}
		return
	}

	opts := models.FindTrackedTimesOptions{
		RepositoryID: ctx.Repo.Repository.ID,
		IssueID:      issue.ID,
	}

	if !ctx.IsUserRepoAdmin() && !ctx.User.IsAdmin {
		opts.UserID = ctx.User.ID
	}

	trackedTimes, err := models.GetTrackedTimes(opts)
	if err != nil {
		ctx.Error(500, "GetTrackedTimes", err)
		return
	}
	ctx.JSON(200, trackedTimes.APIFormat())
}

// EditTime adds/remove time manual to the given issue
func EditTime(ctx *context.APIContext, form api.EditTimeOption) {
	// swagger:operation Post /repos/{owner}/{repo}/issues/{index}/times issue issueEditTime
	// ---
	// summary: Edit the tracked time to a issue
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
	//   description: index of the issue to add tracked time to
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditTimeOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/TrackedTime"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/error"
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(500, "GetIssueByIndex", err)
		}
		return
	}

	if !ctx.Repo.CanUseTimetracker(issue, ctx.User) {
		if !ctx.Repo.Repository.IsTimetrackerEnabled() {
			ctx.JSON(400, struct{ Message string }{Message: "time tracking disabled"})
			return
		}
		ctx.Status(403)
		return
	}

	value := form.Time

	if form.Negative {
		availableTime, err := models.GetTrackedSeconds(models.FindTrackedTimesOptions{
			IssueID: issue.ID,
			UserID:  ctx.User.ID,
		})
		if err != nil {
			ctx.Error(500, "GetTrackedSecounds", err)
			return
		}
		if value > availableTime {
			ctx.Error(403, "EditTime: forbidden to remove more time on issue than you have", nil)
			return
		}
		value *= -1
	}

	trackedTime, err := models.AddTime(ctx.User, issue, value)
	if err != nil {
		ctx.Error(500, "AddTime", err)
		return
	}
	ctx.JSON(200, trackedTime.APIFormat())
}

// ResetIssueTime reset time manual to the given issue
func ResetIssueTime(ctx *context.APIContext) {
	// swagger:operation Delete /repos/{owner}/{repo}/issues/{index}/times issue issueResetTime
	// ---
	// summary: Reset a tracked time of an issue
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
	//   description: index of the issue to add tracked time to
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/empty"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/error"
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(500, "GetIssueByIndex", err)
		}
		return
	}

	if !ctx.Repo.CanUseTimetracker(issue, ctx.User) {
		if !ctx.Repo.Repository.IsTimetrackerEnabled() {
			ctx.JSON(400, struct{ Message string }{Message: "time tracking disabled"})
			return
		}
		ctx.Status(403)
		return
	}

	err = models.DeleteIssueUserTimes(issue, ctx.User)
	if err != nil {
		ctx.Error(500, "DeleteIssueUserTimes", err)
		return
	}
	ctx.Status(200)
}

// ListTrackedTimesByUser  lists all tracked times of the user
func ListTrackedTimesByUser(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/times/{user} user userTrackedTimes
	// ---
	// summary: List a user's tracked times in a repo
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
	// - name: user
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/TrackedTimeList"
	if !ctx.Repo.Repository.IsTimetrackerEnabled() {
		ctx.JSON(400, struct{ Message string }{Message: "time tracking disabled"})
		return
	}
	user, err := models.GetUserByName(ctx.Params(":timetrackingusername"))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(500, "GetUserByName", err)
		}
		return
	}
	if user == nil {
		ctx.NotFound()
		return
	}
	trackedTimes, err := models.GetTrackedTimes(models.FindTrackedTimesOptions{
		UserID:       user.ID,
		RepositoryID: ctx.Repo.Repository.ID})
	if err != nil {
		ctx.Error(500, "GetTrackedTimesByUser", err)
		return
	}
	ctx.JSON(200, trackedTimes.APIFormat())
}

// ListTrackedTimesByRepository lists all tracked times of the repository
func ListTrackedTimesByRepository(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/times repository repoTrackedTimes
	// ---
	// summary: List a repo's tracked times
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
	//     "$ref": "#/responses/TrackedTimeList"
	if !ctx.Repo.Repository.IsTimetrackerEnabled() {
		ctx.JSON(400, struct{ Message string }{Message: "time tracking disabled"})
		return
	}

	opts := models.FindTrackedTimesOptions{
		RepositoryID: ctx.Repo.Repository.ID,
	}

	if !ctx.IsUserRepoAdmin() && !ctx.User.IsAdmin {
		opts.UserID = ctx.User.ID
	}

	trackedTimes, err := models.GetTrackedTimes(opts)
	if err != nil {
		ctx.Error(500, "GetTrackedTimes", err)
		return
	}
	ctx.JSON(200, trackedTimes.APIFormat())
}

// ListMyTrackedTimes lists all tracked times of the current user
func ListMyTrackedTimes(ctx *context.APIContext) {
	// swagger:operation GET /user/times user userCurrentTrackedTimes
	// ---
	// summary: List the current user's tracked times
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/TrackedTimeList"
	trackedTimes, err := models.GetTrackedTimes(models.FindTrackedTimesOptions{UserID: ctx.User.ID})
	if err != nil {
		ctx.Error(500, "GetTrackedTimesByUser", err)
		return
	}
	ctx.JSON(200, trackedTimes.APIFormat())
}
