// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// AddIssueSubscription Subscribe user to issue
func AddIssueSubscription(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/issues/{index}/subscriptions/{user} issue issueAddSubscription
	// ---
	// summary: Subscribe user to issue
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
	//   description: index of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// - name: user
	//   in: path
	//   description: user to subscribe
	//   type: string
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/empty"
	//   "304":
	//     description: User can only subscribe itself if he is no admin
	//   "404":
	//     "$ref": "#/responses/notFound"

	setIssueSubscription(ctx, true)
}

// DelIssueSubscription Unsubscribe user from issue
func DelIssueSubscription(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/{index}/subscriptions/{user} issue issueDeleteSubscription
	// ---
	// summary: Unsubscribe user from issue
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
	//   description: index of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// - name: user
	//   in: path
	//   description: user witch unsubscribe
	//   type: string
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/empty"
	//   "304":
	//     description: User can only subscribe itself if he is no admin
	//   "404":
	//     "$ref": "#/responses/notFound"

	setIssueSubscription(ctx, false)
}

func setIssueSubscription(ctx *context.APIContext, watch bool) {
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}

		return
	}

	user, err := models.GetUserByName(ctx.Params(":user"))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
		}

		return
	}

	//only admin and user for itself can change subscription
	if user.ID != ctx.User.ID && !ctx.User.IsAdmin {
		ctx.Error(http.StatusForbidden, "User", nil)
		return
	}

	if err := models.CreateOrUpdateIssueWatch(user.ID, issue.ID, watch); err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateOrUpdateIssueWatch", err)
		return
	}

	ctx.Status(http.StatusCreated)
}

// GetIssueSubscribers return subscribers of an issue
func GetIssueSubscribers(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{index}/subscriptions issue issueSubscriptions
	// ---
	// summary: Get users who subscribed on an issue.
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
	//   description: index of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}

		return
	}

	iwl, err := models.GetIssueWatchers(issue.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetIssueWatchers", err)
		return
	}

	users, err := iwl.LoadWatchUsers()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadWatchUsers", err)
		return
	}

	ctx.JSON(http.StatusOK, users.APIFormat())
}
