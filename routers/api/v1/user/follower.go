// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
)

func responseAPIUsers(ctx *context.APIContext, users []*models.User) {
	apiUsers := make([]*api.User, len(users))
	for i := range users {
		apiUsers[i] = convert.ToUser(users[i], ctx.IsSigned, ctx.User != nil && ctx.User.IsAdmin)
	}
	ctx.JSON(http.StatusOK, &apiUsers)
}

func listUserFollowers(ctx *context.APIContext, u *models.User) {
	users, err := u.GetFollowers(ctx.QueryInt("page"))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUserFollowers", err)
		return
	}
	responseAPIUsers(ctx, users)
}

// ListMyFollowers list the authenticated user's followers
func ListMyFollowers(ctx *context.APIContext) {
	// swagger:operation GET /user/followers user userCurrentListFollowers
	// ---
	// summary: List the authenticated user's followers
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"

	listUserFollowers(ctx, ctx.User)
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"

	u := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	listUserFollowers(ctx, u)
}

func listUserFollowing(ctx *context.APIContext, u *models.User) {
	users, err := u.GetFollowing(ctx.QueryInt("page"))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetFollowing", err)
		return
	}
	responseAPIUsers(ctx, users)
}

// ListMyFollowing list the users that the authenticated user is following
func ListMyFollowing(ctx *context.APIContext) {
	// swagger:operation GET /user/following user userCurrentListFollowing
	// ---
	// summary: List the users that the authenticated user is following
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"

	listUserFollowing(ctx, ctx.User)
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"

	u := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	listUserFollowing(ctx, u)
}

func checkUserFollowing(ctx *context.APIContext, u *models.User, followID int64) {
	if u.IsFollowing(followID) {
		ctx.Status(http.StatusNoContent)
	} else {
		ctx.NotFound()
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

	target := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	checkUserFollowing(ctx, ctx.User, target.ID)
}

// CheckFollowing check if one user is following another user
func CheckFollowing(ctx *context.APIContext) {
	// swagger:operation GET /users/{follower}/following/{followee} user userCheckFollowing
	// ---
	// summary: Check if one user is following another user
	// parameters:
	// - name: follower
	//   in: path
	//   description: username of following user
	//   type: string
	//   required: true
	// - name: followee
	//   in: path
	//   description: username of followed user
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	u := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	target := GetUserByParamsName(ctx, ":target")
	if ctx.Written() {
		return
	}
	checkUserFollowing(ctx, u, target.ID)
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

	target := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if err := models.FollowUser(ctx.User.ID, target.ID); err != nil {
		ctx.Error(http.StatusInternalServerError, "FollowUser", err)
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

	target := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if err := models.UnfollowUser(ctx.User.ID, target.ID); err != nil {
		ctx.Error(http.StatusInternalServerError, "UnfollowUser", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
