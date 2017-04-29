// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

func responseAPIUsers(ctx *context.APIContext, users []*models.User) {
	apiUsers := make([]*api.User, len(users))
	for i := range users {
		apiUsers[i] = users[i].APIFormat()
	}
	ctx.JSON(200, &apiUsers)
}

func listUserFollowers(ctx *context.APIContext, u *models.User) {
	users, err := u.GetFollowers(ctx.QueryInt("page"))
	if err != nil {
		ctx.Error(500, "GetUserFollowers", err)
		return
	}
	responseAPIUsers(ctx, users)
}

// ListMyFollowers list all my followers
func ListMyFollowers(ctx *context.APIContext) {
	// swagger:route GET /user/followers userCurrentListFollowers
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: UserList
	//       500: error

	listUserFollowers(ctx, ctx.User)
}

// ListFollowers list user's followers
func ListFollowers(ctx *context.APIContext) {
	// swagger:route GET /users/:username/followers userListFollowers
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: UserList
	//       500: error

	u := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	listUserFollowers(ctx, u)
}

func listUserFollowing(ctx *context.APIContext, u *models.User) {
	users, err := u.GetFollowing(ctx.QueryInt("page"))
	if err != nil {
		ctx.Error(500, "GetFollowing", err)
		return
	}
	responseAPIUsers(ctx, users)
}

// ListMyFollowing list all my followings
func ListMyFollowing(ctx *context.APIContext) {
	// swagger:route GET /user/following userCurrentListFollowing
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: UserList
	//       500: error

	listUserFollowing(ctx, ctx.User)
}

// ListFollowing list user's followings
func ListFollowing(ctx *context.APIContext) {
	// swagger:route GET /users/{username}/following userListFollowing
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: UserList
	//       500: error

	u := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	listUserFollowing(ctx, u)
}

func checkUserFollowing(ctx *context.APIContext, u *models.User, followID int64) {
	if u.IsFollowing(followID) {
		ctx.Status(204)
	} else {
		ctx.Status(404)
	}
}

// CheckMyFollowing check if the repo is followed by me
func CheckMyFollowing(ctx *context.APIContext) {
	// swagger:route GET /user/following/{username} userCurrentCheckFollowing
	//
	//     Responses:
	//       204: empty
	//       404: notFound

	target := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	checkUserFollowing(ctx, ctx.User, target.ID)
}

// CheckFollowing check if the repo is followed by user
func CheckFollowing(ctx *context.APIContext) {
	// swagger:route GET /users/{username}/following/:target userCheckFollowing
	//
	//     Responses:
	//       204: empty
	//       404: notFound

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

// Follow follow one repository
func Follow(ctx *context.APIContext) {
	// swagger:route PUT /user/following/{username} userCurrentPutFollow
	//
	//     Responses:
	//       204: empty
	//       500: error

	target := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if err := models.FollowUser(ctx.User.ID, target.ID); err != nil {
		ctx.Error(500, "FollowUser", err)
		return
	}
	ctx.Status(204)
}

// Unfollow unfollow one repository
func Unfollow(ctx *context.APIContext) {
	// swagger:route DELETE /user/following/{username} userCurrentDeleteFollow
	//
	//     Responses:
	//       204: empty
	//       500: error

	target := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if err := models.UnfollowUser(ctx.User.ID, target.ID); err != nil {
		ctx.Error(500, "UnfollowUser", err)
		return
	}
	ctx.Status(204)
}
