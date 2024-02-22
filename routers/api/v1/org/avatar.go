// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"encoding/base64"
	"net/http"

	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	user_service "code.gitea.io/gitea/services/user"
)

// UpdateAvatarupdates the Avatar of an Organisation
func UpdateAvatar(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/avatar organization orgUpdateAvatar
	// ---
	// summary: Update Avatar
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/UpdateUserAvatarOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/ByteSlice"
	//   "201":
	//     "$ref": "#/responses/ByteSlice"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	form := web.GetForm(ctx).(*api.UpdateUserAvatarOption)

	content, err := base64.StdEncoding.DecodeString(form.Image)
	if err != nil {
		ctx.Error(http.StatusBadRequest, "DecodeImage", err)
		return
	}

	user := ctx.Org.Organization.AsUser()
	hasAvatar := user.HasAvatar()

	avatarData, err := user_service.UploadAvatar(ctx, user, content)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "UploadAvatar", err)
		return
	}

	if hasAvatar {
		ctx.Status(http.StatusOK)
	} else {
		ctx.Status(http.StatusCreated)
	}
	_, err = ctx.Write(avatarData)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Write", err)
		return
	}
}

// DeleteAvatar deletes the Avatar of an Organisation
func DeleteAvatar(ctx *context.APIContext) {
	// swagger:operation DELETE /orgs/{org}/avatar organization orgDeleteAvatar
	// ---
	// summary: Delete Avatar
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	err := user_service.DeleteAvatar(ctx, ctx.Org.Organization.AsUser())
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteAvatar", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
