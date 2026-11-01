// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	user_service "gitea.dev/services/user"
)

// GetUserSettings returns user settings
func GetUserSettings(ctx *context.APIContext) {
	// swagger:operation GET /user/settings user getUserSettings
	// ---
	// summary: Get user settings
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserSettings"
	ctx.JSON(http.StatusOK, convert.User2UserSettings(ctx.Doer))
}

// UpdateUserSettings returns user settings
func UpdateUserSettings(ctx *context.APIContext) {
	// swagger:operation PATCH /user/settings user updateUserSettings
	// ---
	// summary: Update user settings
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/UserSettingsOptions"
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserSettings"
	//   "404":
	//     "$ref": "#/responses/notFound"

	form := web.GetForm[*api.UserSettingsOptions](ctx)
	if form.FullName != nil && *form.FullName != ctx.Doer.FullName && user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureChangeFullName) {
		ctx.APIErrorNotFound("full name is not allowed to be changed")
		return
	}
	if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureChangeActivityVisibility) {
		if form.HideActivity != nil && *form.HideActivity {
			ctx.APIErrorNotFound("activity visibility is not allowed to be changed")
			return
		}
		form.HideActivity = nil // keep the stored preference for when the lock is lifted
	}

	opts := &user_service.UpdateOptions{
		FullName:            optional.FromPtr(form.FullName),
		Description:         optional.FromPtr(form.Description),
		Website:             optional.FromPtr(form.Website),
		Location:            optional.FromPtr(form.Location),
		Language:            optional.FromPtr(form.Language),
		Theme:               optional.FromPtr(form.Theme),
		DiffViewStyle:       optional.FromPtr(form.DiffViewStyle),
		KeepEmailPrivate:    optional.FromPtr(form.HideEmail),
		KeepActivityPrivate: optional.FromPtr(form.HideActivity),
	}
	if err := user_service.UpdateUser(ctx, ctx.Doer, opts); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.User2UserSettings(ctx.Doer))
}
