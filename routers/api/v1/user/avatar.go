// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"encoding/base64"
	"net/http"

	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	user_service "gitea.dev/services/user"
)

// UpdateAvatar updates the Avatar of a User
func UpdateAvatar(ctx *context.APIContext) {
	// swagger:operation POST /user/avatar user userUpdateAvatar
	// ---
	// summary: Update Avatar
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/UpdateUserAvatarOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	form := web.GetForm(ctx).(*api.UpdateUserAvatarOption)

	content, err := base64.StdEncoding.DecodeString(form.Image)
	if err != nil {
		ctx.APIError(http.StatusBadRequest, err)
		return
	}

	err = user_service.UploadAvatar(ctx, ctx.Doer, content)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// DeleteAvatar deletes the Avatar of a User
func DeleteAvatar(ctx *context.APIContext) {
	// swagger:operation DELETE /user/avatar user userDeleteAvatar
	// ---
	// summary: Delete Avatar
	// produces:
	// - application/json
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	err := user_service.DeleteAvatar(ctx, ctx.Doer)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
