// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"

	api "code.gitea.io/sdk/gitea"
)

func trackedTimesToAPIFormat(trackedTimes []*models.TrackedTime) []*api.TrackedTime {
	apiTrackedTimes := make([]*api.TrackedTime, len(trackedTimes))
	for i, trackedTime := range trackedTimes {
		apiTrackedTimes[i] = trackedTime.APIFormat()
	}
	return apiTrackedTimes
}

// ListTrackedTimes list all the tracked times of an issue
func ListTrackedTimes(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{id}/times issue issueTrackedTimes
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
	// - name: id
	//   in: path
	//   description: index of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/TrackedTimeList"
	if !ctx.Repo.Repository.IsTimetrackerEnabled() {
		ctx.Error(404, "IsTimetrackerEnabled", "Timetracker is diabled")
		return
	}
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.Error(404, "GetIssueByIndex", err)
		} else {
			ctx.Error(500, "GetIssueByIndex", err)
		}
		return
	}

	trackedTimes, err := models.GetTrackedTimes(models.FindTrackedTimesOptions{IssueID: issue.ID})
	if err != nil {
		ctx.Error(500, "GetTrackedTimesByIssue", err)
		return
	}
	apiTrackedTimes := trackedTimesToAPIFormat(trackedTimes)
	ctx.JSON(200, &apiTrackedTimes)
}

// AddTime adds time manual to the given issue
func AddTime(ctx *context.APIContext, form api.AddTimeOption) {
	// swagger:operation Post /repos/{owner}/{repo}/issues/{id}/times issue issueAddTime
	// ---
	// summary: Add a tracked time to a issue
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
	// - name: id
	//   in: path
	//   description: index of the issue to add tracked time to
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/AddTimeOption"
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
			ctx.Error(404, "GetIssueByIndex", err)
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
	trackedTime, err := models.AddTime(ctx.User, issue, form.Time)
	if err != nil {
		ctx.Error(500, "AddTime", err)
		return
	}
	ctx.JSON(200, trackedTime.APIFormat())
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
			ctx.Error(404, "GetUserByName", err)
		} else {
			ctx.Error(500, "GetUserByName", err)
		}
		return
	}
	if user == nil {
		ctx.Status(404)
		return
	}
	trackedTimes, err := models.GetTrackedTimes(models.FindTrackedTimesOptions{
		UserID:       user.ID,
		RepositoryID: ctx.Repo.Repository.ID})
	if err != nil {
		ctx.Error(500, "GetTrackedTimesByUser", err)
		return
	}
	apiTrackedTimes := trackedTimesToAPIFormat(trackedTimes)
	ctx.JSON(200, &apiTrackedTimes)
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
	trackedTimes, err := models.GetTrackedTimes(models.FindTrackedTimesOptions{
		RepositoryID: ctx.Repo.Repository.ID})
	if err != nil {
		ctx.Error(500, "GetTrackedTimesByUser", err)
		return
	}
	apiTrackedTimes := trackedTimesToAPIFormat(trackedTimes)
	ctx.JSON(200, &apiTrackedTimes)
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
	apiTrackedTimes := trackedTimesToAPIFormat(trackedTimes)
	ctx.JSON(200, &apiTrackedTimes)
}
