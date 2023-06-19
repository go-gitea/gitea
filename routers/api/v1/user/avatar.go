// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/web"
	user_service "code.gitea.io/gitea/services/user"
)

// UpdateAvatar updates the Avatar of an User
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
		ctx.Error(http.StatusBadRequest, "DecodeImage", err)
		return
	}

	if int64(len(content)) > setting.Avatar.MaxFileSize {
		ctx.Error(http.StatusBadRequest, "AvatarTooBig", fmt.Errorf("The avatar is to big"))
		return
	}

	st := typesniffer.DetectContentType(content)
	if !(st.IsImage() && !st.IsSvgImage()) {
		ctx.Error(http.StatusBadRequest, "NotAnImage", fmt.Errorf("The avatar is not an image"))
		return
	}

	err = user_service.UploadAvatar(ctx.Doer, content)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "UploadAvatar", err)
	}

	ctx.Status(http.StatusNoContent)
}

// DeleteAvatar deletes the Avatar of an User
func DeleteAvatar(ctx *context.APIContext) {
	// swagger:operation DELETE /user/avatar user userDeleteAvatar
	// ---
	// summary: Delete Avatar
	// produces:
	// - application/json
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	err := user_service.DeleteAvatar(ctx.Doer)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteAvatar", err)
	}

	ctx.Status(http.StatusNoContent)
}
