// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
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
	//   "200":
	//     description: Already subscribed
	//   "201":
	//     description: Successfully Subscribed
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
	//   "200":
	//     description: Already unsubscribed
	//   "201":
	//     description: Successfully Unsubscribed
	//   "304":
	//     description: User can only subscribe itself if he is no admin
	//   "404":
	//     "$ref": "#/responses/notFound"

	setIssueSubscription(ctx, false)
}

func setIssueSubscription(ctx *context.APIContext, watch bool) {
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}

		return
	}

	user, err := user_model.GetUserByName(ctx, ctx.PathParam("user"))
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
		}

		return
	}

	// only admin and user for itself can change subscription
	if user.ID != ctx.Doer.ID && !ctx.Doer.IsAdmin {
		ctx.Error(http.StatusForbidden, "User", fmt.Errorf("%s is not permitted to change subscriptions for %s", ctx.Doer.Name, user.Name))
		return
	}

	current, err := issues_model.CheckIssueWatch(ctx, user, issue)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CheckIssueWatch", err)
		return
	}

	// If watch state wont change
	if current == watch {
		ctx.Status(http.StatusOK)
		return
	}

	// Update watch state
	if err := issues_model.CreateOrUpdateIssueWatch(ctx, user.ID, issue.ID, watch); err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateOrUpdateIssueWatch", err)
		return
	}

	ctx.Status(http.StatusCreated)
}

// CheckIssueSubscription check if user is subscribed to an issue
func CheckIssueSubscription(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{index}/subscriptions/check issue issueCheckSubscription
	// ---
	// summary: Check if user is subscribed to an issue
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
	//     "$ref": "#/responses/WatchInfo"
	//   "404":
	//     "$ref": "#/responses/notFound"

	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}

		return
	}

	watching, err := issues_model.CheckIssueWatch(ctx, ctx.Doer, issue)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	ctx.JSON(http.StatusOK, api.WatchInfo{
		Subscribed:    watching,
		Ignored:       !watching,
		Reason:        nil,
		CreatedAt:     issue.CreatedUnix.AsTime(),
		URL:           issue.APIURL(ctx) + "/subscriptions",
		RepositoryURL: ctx.Repo.Repository.APIURL(),
	})
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
	//     "$ref": "#/responses/UserList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}

		return
	}

	iwl, err := issues_model.GetIssueWatchers(ctx, issue.ID, utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetIssueWatchers", err)
		return
	}

	userIDs := make([]int64, 0, len(iwl))
	for _, iw := range iwl {
		userIDs = append(userIDs, iw.UserID)
	}

	users, err := user_model.GetUsersByIDs(ctx, userIDs)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUsersByIDs", err)
		return
	}
	apiUsers := make([]*api.User, 0, len(users))
	for _, v := range users {
		apiUsers = append(apiUsers, convert.ToUser(ctx, v, ctx.Doer))
	}

	count, err := issues_model.CountIssueWatchers(ctx, issue.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CountIssueWatchers", err)
		return
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, apiUsers)
}
