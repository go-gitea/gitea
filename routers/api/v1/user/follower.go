// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package user

import (
	"errors"
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

func responseAPIUsers(ctx *context.APIContext, users []*user_model.User) {
	apiUsers := make([]*api.User, len(users))
	for i := range users {
		apiUsers[i] = convert.ToUser(ctx, users[i], ctx.Doer)
	}
	ctx.JSON(http.StatusOK, &apiUsers)
}

func listUserFollowers(ctx *context.APIContext, u *user_model.User) {
	users, count, err := user_model.GetUserFollowers(ctx, u, ctx.Doer, utils.GetListOptions(ctx))
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetTotalCountHeader(count)
	responseAPIUsers(ctx, users)
}

// ListMyFollowers list the authenticated user's followers
func ListMyFollowers(ctx *context.APIContext) {
	// swagger:operation GET /user/followers user userCurrentListFollowers
	// ---
	// summary: List the authenticated user's followers
	// parameters:
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"

	listUserFollowers(ctx, ctx.Doer)
}

// ListFollowers list the given user's followers
func ListFollowers(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/followers user userListFollowers
	// ---
	// summary: List the given user's followers
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
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

	listUserFollowers(ctx, ctx.ContextUser)
}

func listUserFollowing(ctx *context.APIContext, u *user_model.User) {
	users, count, err := user_model.GetUserFollowing(ctx, u, ctx.Doer, utils.GetListOptions(ctx))
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetTotalCountHeader(count)
	responseAPIUsers(ctx, users)
}

// ListMyFollowing list the users that the authenticated user is following
func ListMyFollowing(ctx *context.APIContext) {
	// swagger:operation GET /user/following user userCurrentListFollowing
	// ---
	// summary: List the users that the authenticated user is following
	// parameters:
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"

	listUserFollowing(ctx, ctx.Doer)
}

// ListFollowing list the users that the given user is following
func ListFollowing(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/following user userListFollowing
	// ---
	// summary: List the users that the given user is following
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
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

	listUserFollowing(ctx, ctx.ContextUser)
}

func checkUserFollowing(ctx *context.APIContext, u *user_model.User, followID int64) {
	if user_model.IsFollowing(ctx, u.ID, followID) {
		ctx.Status(http.StatusNoContent)
	} else {
		ctx.APIErrorNotFound()
	}
}

// CheckMyFollowing whether the given user is followed by the authenticated user
func CheckMyFollowing(ctx *context.APIContext) {
	// swagger:operation GET /user/following/{username} user userCurrentCheckFollowing
	// ---
	// summary: Check whether a user is followed by the authenticated user
	// parameters:
	// - name: username
	//   in: path
	//   description: username of followed user
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	checkUserFollowing(ctx, ctx.Doer, ctx.ContextUser.ID)
}

// CheckFollowing check if one user is following another user
func CheckFollowing(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/following/{target} user userCheckFollowing
	// ---
	// summary: Check if one user is following another user
	// parameters:
	// - name: username
	//   in: path
	//   description: username of following user
	//   type: string
	//   required: true
	// - name: target
	//   in: path
	//   description: username of followed user
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	target := GetUserByPathParam(ctx, "target") // FIXME: it is not right to call this function, it should load the "target" directly
	if ctx.Written() {
		return
	}
	checkUserFollowing(ctx, ctx.ContextUser, target.ID)
}

// Follow follow a user
func Follow(ctx *context.APIContext) {
	// swagger:operation PUT /user/following/{username} user userCurrentPutFollow
	// ---
	// summary: Follow a user
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user to follow
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if err := user_model.FollowUser(ctx, ctx.Doer, ctx.ContextUser); err != nil {
		if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.APIError(http.StatusForbidden, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}

// Unfollow unfollow a user
func Unfollow(ctx *context.APIContext) {
	// swagger:operation DELETE /user/following/{username} user userCurrentDeleteFollow
	// ---
	// summary: Unfollow a user
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user to unfollow
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if err := user_model.UnfollowUser(ctx, ctx.Doer.ID, ctx.ContextUser.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
