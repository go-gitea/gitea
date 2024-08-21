// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
)

// ListUserBadges lists all badges belonging to a user
func ListUserBadges(ctx *context.APIContext) {
	// swagger:operation GET /admin/users/{username}/badges admin adminListUserBadges
	// ---
	// summary: List a user's badges
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
	//     "$ref": "#/responses/BadgeList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	badges, maxResults, err := user_model.GetUserBadges(ctx, ctx.ContextUser)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUserBadges", err)
		return
	}

	ctx.SetTotalCountHeader(maxResults)
	ctx.JSON(http.StatusOK, &badges)
}

// AddUserBadges add badges to a user
func AddUserBadges(ctx *context.APIContext) {
	// swagger:operation POST /admin/users/{username}/badges admin adminAddUserBadges
	// ---
	// summary: Add a badge to a user
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/UserBadgeOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	form := web.GetForm(ctx).(*api.UserBadgeOption)
	badges := prepareBadgesForReplaceOrAdd(*form)

	if err := user_model.AddUserBadges(ctx, ctx.ContextUser, badges); err != nil {
		ctx.Error(http.StatusInternalServerError, "ReplaceUserBadges", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// DeleteUserBadges delete a badge from a user
func DeleteUserBadges(ctx *context.APIContext) {
	// swagger:operation DELETE /admin/users/{username}/badges admin adminDeleteUserBadges
	// ---
	// summary: Remove a badge from a user
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/UserBadgeOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.UserBadgeOption)
	badges := prepareBadgesForReplaceOrAdd(*form)

	if err := user_model.RemoveUserBadges(ctx, ctx.ContextUser, badges); err != nil {
		ctx.Error(http.StatusInternalServerError, "ReplaceUserBadges", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func prepareBadgesForReplaceOrAdd(form api.UserBadgeOption) []*user_model.Badge {
	badges := make([]*user_model.Badge, len(form.BadgeSlugs))
	for i, badge := range form.BadgeSlugs {
		badges[i] = &user_model.Badge{
			Slug: badge,
		}
	}
	return badges
}
