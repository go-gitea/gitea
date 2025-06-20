// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/context/upload"
	files_service "code.gitea.io/gitea/services/repository/files"
)

// UploadFileToServer upload file to server file dir not git
func UploadFileToServer(ctx *context.Context) {
	file, header, err := ctx.Req.FormFile("file")
	if err != nil {
		ctx.ServerError("FormFile", err)
		return
	}
	defer file.Close()

	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(file, buf)
	if n > 0 {
		buf = buf[:n]
	}

	err = upload.Verify(buf, header.Filename, setting.Repository.Upload.AllowedTypes)
	if err != nil {
		ctx.HTTPError(http.StatusBadRequest, err.Error())
		return
	}

	name := files_service.CleanGitTreePath(header.Filename)
	if len(name) == 0 {
		ctx.HTTPError(http.StatusBadRequest, "Upload file name is invalid")
		return
	}

	uploaded, err := repo_model.NewUpload(ctx, name, buf, file)
	if err != nil {
		ctx.ServerError("NewUpload", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{"uuid": uploaded.UUID})
}

// RemoveUploadFileFromServer remove file from server file dir
func RemoveUploadFileFromServer(ctx *context.Context) {
	fileUUID := ctx.FormString("file")
	if err := repo_model.DeleteUploadByUUID(ctx, fileUUID); err != nil {
		ctx.ServerError("DeleteUploadByUUID", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
