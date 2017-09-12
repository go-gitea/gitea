// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/sdk/gitea"
)

// ListTrackedTimes list all the tracked times of an issue
func ListTrackedTimes(ctx *context.APIContext) {
	// swagger:route GET /repos/{username}/{reponame}/issues/{issue}/times repository issueTrackedTimes
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: TrackedTimes
	//	 404: error
	//       500: error
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

	if trackedTimes, err := models.GetTrackedTimes(models.FindTrackedTimesOptions{IssueID: issue.ID}); err != nil {
		ctx.Error(500, "GetTrackedTimesByIssue", err)
	} else {
		ctx.JSON(200, &trackedTimes)
	}
}

// AddTime adds time manual to the given issue
func AddTime(ctx *context.APIContext, form api.AddTimeOption) {
	// swagger:route Post /repos/{username}/{reponame}/issues/{issue}/times repository addTime
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: TrackedTime
	//       400: error
	//       403: error
	//	 404: error
	//       500: error
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
	var tt *models.TrackedTime
	if tt, err = models.AddTime(ctx.User, issue, form.Time); err != nil {
		ctx.Error(500, "AddTime", err)
		return
	}
	ctx.JSON(200, tt)

}

// ListTrackedTimesByUser  lists all tracked times of the user
func ListTrackedTimesByUser(ctx *context.APIContext) {
	// swagger:route GET /repos/{username}/{reponame}/times/{timetrackingusername} user userTrackedTimes
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: TrackedTimes
	//       400: error
	//	 404: error
	//       500: error
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
	if trackedTimes, err := models.GetTrackedTimes(models.FindTrackedTimesOptions{UserID: user.ID, RepositoryID: ctx.Repo.Repository.ID}); err != nil {
		ctx.Error(500, "GetTrackedTimesByUser", err)
	} else {
		ctx.JSON(200, &trackedTimes)
	}
}

// ListTrackedTimesByRepository lists all tracked times of the user
func ListTrackedTimesByRepository(ctx *context.APIContext) {
	// swagger:route GET /repos/{username}/{reponame}/times repository repoTrackedTimes
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: TrackedTimes
	//       400: error
	//       500: error
	if !ctx.Repo.Repository.IsTimetrackerEnabled() {
		ctx.JSON(400, struct{ Message string }{Message: "time tracking disabled"})
		return
	}
	if trackedTimes, err := models.GetTrackedTimes(models.FindTrackedTimesOptions{RepositoryID: ctx.Repo.Repository.ID}); err != nil {
		ctx.Error(500, "GetTrackedTimesByUser", err)
	} else {
		ctx.JSON(200, &trackedTimes)
	}
}

// ListMyTrackedTimes lists all tracked times of the current user
func ListMyTrackedTimes(ctx *context.APIContext) {
	// swagger:route GET /user/times user userTrackedTimes
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: TrackedTimes
	//       500: error
	if trackedTimes, err := models.GetTrackedTimes(models.FindTrackedTimesOptions{UserID: ctx.User.ID}); err != nil {
		ctx.Error(500, "GetTrackedTimesByUser", err)
	} else {
		ctx.JSON(200, &trackedTimes)
	}
}
