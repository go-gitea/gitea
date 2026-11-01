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

	form := web.GetForm[*api.UserSettingsOptions](ctx)

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
	if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureChangeActivityVisibility) {
		opts.KeepActivityPrivate = optional.None[bool]()
	}
	if err := user_service.UpdateUser(ctx, ctx.Doer, opts); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.User2UserSettings(ctx.Doer))
}
