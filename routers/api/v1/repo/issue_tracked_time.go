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
	// swagger:route GET /repos/{username}/{reponame}/issues/{issue}/times issueTrackedTimes
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

	if trackedTimes, err := models.GetTrackedTimesByIssue(issue.ID); err != nil {
		ctx.Error(500, "GetTrackedTimesByIssue", err)
	} else {
		ctx.JSON(200, &trackedTimes)
	}
}

// AddTime adds time manual to the given issue
func AddTime(ctx *context.APIContext, form api.AddTimeOption) {
	// swagger:route Post /repos/{username}/{reponame}/issues/{issue}/times addTime
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: AddTimeOption
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
		ctx.Status(403)
		return
	}

	if err := models.AddTime(ctx.User.ID, issue.ID, form.Time); err != nil {
		ctx.Error(500, "AddTime", err)
		return
	}
	ctx.JSON(200, form)
}

// ListTrackedTimesByUser  lists all tracked times of the user
func ListTrackedTimesByUser(ctx *context.APIContext) {
	// swagger:route GET /user/{username}/times userTrackedTimes
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: TrackedTimes
	//	 404: error
	//       500: error
	user := GetUserByParamsName(ctx, ctx.Params(":username"))
	if user == nil {
		return
	}

	if trackedTimes, err := models.GetTrackedTimesByUser(user.ID); err != nil {
		ctx.Error(500, "GetTrackedTimesByUser", err)
	} else {
		ctx.JSON(200, &trackedTimes)
	}
}

// ListMyTrackedTimes lists all tracked times of the current user
func ListMyTrackedTimes(ctx *context.APIContext) {
	// swagger:route GET /user/times userTrackedTimes
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: TrackedTimes
	//       500: error
	if trackedTimes, err := models.GetTrackedTimesByUser(ctx.User.ID); err != nil {
		ctx.Error(500, "GetTrackedTimesByUser", err)
	} else {
		ctx.JSON(200, &trackedTimes)
	}
}

// GetUserByParamsName get user by name
func GetUserByParamsName(ctx *context.APIContext, name string) *models.User {
	user, err := models.GetUserByName(ctx.Params(name))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Error(404, "GetUserByName", err)
		} else {
			ctx.Error(500, "GetUserByName", err)
		}
		return nil
	}
	return user
}
