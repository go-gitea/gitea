// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/convert"
)

// ListSubscribers list a repo's subscribers (i.e. watchers)
func ListSubscribers(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/subscribers repository repoListSubscribers
	// ---
	// summary: List a repo's watchers
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

	subscribers, err := repo_model.GetRepoWatchers(ctx, ctx.Repo.Repository.ID, utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetRepoWatchers", err)
		return
	}
	users := make([]*api.User, len(subscribers))
	for i, subscriber := range subscribers {
		users[i] = convert.ToUser(ctx, subscriber, ctx.Doer)
	}

	ctx.SetTotalCountHeader(int64(ctx.Repo.Repository.NumWatches))
	ctx.JSON(http.StatusOK, users)
}

// ListSubscribers list a repo's subscribers (i.e. watchers) for the given event
func ListSubscribersEvent(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/subscribers/{event} repository repoListSubscribersEvent
	// ---
	// summary: List a repo's watchers for the given event
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
	// - name: event
	//   in: path
	//   description: the event
	//   type: string
	//   enum: [issues,pullrequests,releases]
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
	var eventConst repo_model.WatchEventType

	switch strings.ToLower(ctx.Params("event")) {
	case "issues":
		eventConst = repo_model.WatchEventTypeIssue
	case "pullrequests":
		eventConst = repo_model.WatchEventTypePullRequest
	case "releases":
		eventConst = repo_model.WatchEventTypeRelease
	default:
		ctx.Error(http.StatusBadRequest, "InvalidEvent", fmt.Errorf("%s is not a valid event", ctx.Params("event")))
		return
	}

	subscribers, err := repo_model.GetRepoWatchersEvent(ctx.Repo.Repository.ID, eventConst, utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetRepoWatchersEvent", err)
		return
	}
	users := make([]*api.User, len(subscribers))
	for i, subscriber := range subscribers {
		users[i] = convert.ToUser(ctx, subscriber, ctx.Doer)
	}

	ctx.SetTotalCountHeader(int64(ctx.Repo.Repository.NumWatches))
	ctx.JSON(http.StatusOK, users)
}
