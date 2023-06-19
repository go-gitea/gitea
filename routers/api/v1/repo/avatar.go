// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/web"
	repo_service "code.gitea.io/gitea/services/repository"
)

// UpdateVatar updates the Avatar of an Repo
func UpdateAvatar(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/avatar repository repoUpdateAvatar
	// ---
	// summary: Update avatar
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
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/UpdateRepoAvatarOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	form := web.GetForm(ctx).(*api.UpdateRepoAvatarOption)

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

	err = repo_service.UploadAvatar(ctx, ctx.Repo.Repository, content)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "UploadAvatar", err)
	}

	ctx.Status(http.StatusNoContent)
}

// UpdateAvatar deletes the Avatar of an Repo
func DeleteAvatar(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/avatar repository repoDeleteAvatar
	// ---
	// summary: Delete avatar
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
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	err := repo_service.DeleteAvatar(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteAvatar", err)
	}

	ctx.Status(http.StatusNoContent)
}
