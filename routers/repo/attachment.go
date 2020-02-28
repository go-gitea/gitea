// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/upload"
)

func renderAttachmentSettings(ctx *context.Context) {
	ctx.Data["RequireDropzone"] = true
	ctx.Data["IsAttachmentEnabled"] = setting.AttachmentEnabled
	ctx.Data["AttachmentAllowedTypes"] = setting.AttachmentAllowedTypes
	ctx.Data["AttachmentMaxSize"] = setting.AttachmentMaxSize
	ctx.Data["AttachmentMaxFiles"] = setting.AttachmentMaxFiles
}

// UploadAttachment response for uploading issue's attachment
func UploadAttachment(ctx *context.Context) {
	if !setting.AttachmentEnabled {
		ctx.Error(404, "attachment is not enabled")
		return
	}

	file, header, err := ctx.Req.FormFile("file")
	if err != nil {
		ctx.Error(500, fmt.Sprintf("FormFile: %v", err))
		return
	}
	defer file.Close()

	buf := make([]byte, 1024)
	n, _ := file.Read(buf)
	if n > 0 {
		buf = buf[:n]
	}

	err = upload.VerifyAllowedContentType(buf, strings.Split(setting.AttachmentAllowedTypes, ","))
	if err != nil {
		ctx.Error(400, err.Error())
		return
	}

	attach, err := models.NewAttachment(&models.Attachment{
		UploaderID: ctx.User.ID,
		Name:       header.Filename,
	}, buf, file)
	if err != nil {
		ctx.Error(500, fmt.Sprintf("NewAttachment: %v", err))
		return
	}

	log.Trace("New attachment uploaded: %s", attach.UUID)
	ctx.JSON(200, map[string]string{
		"uuid": attach.UUID,
	})
}

// DeleteAttachment response for deleting issue's attachment
func DeleteAttachment(ctx *context.Context) {
	file := ctx.Query("file")
	attach, err := models.GetAttachmentByUUID(file)
	if err != nil {
		ctx.Error(400, err.Error())
		return
	}
	if !ctx.IsSigned || (ctx.User.ID != attach.UploaderID) {
		ctx.Error(403)
		return
	}
	err = models.DeleteAttachment(attach, true)
	if err != nil {
		ctx.Error(500, fmt.Sprintf("DeleteAttachment: %v", err))
		return
	}
	ctx.JSON(200, map[string]string{
		"uuid": attach.UUID,
	})
}

// GetAttachment serve attachements
func GetAttachment(ctx *context.Context) {
	attach, err := models.GetAttachmentByUUID(ctx.Params(":uuid"))
	if err != nil {
		if models.IsErrAttachmentNotExist(err) {
			ctx.Error(404)
		} else {
			ctx.ServerError("GetAttachmentByUUID", err)
		}
		return
	}

	repository, unitType, err := attach.LinkedRepository()
	if err != nil {
		ctx.ServerError("LinkedRepository", err)
		return
	}

	if repository == nil { //If not linked
		if !(ctx.IsSigned && attach.UploaderID == ctx.User.ID) { //We block if not the uploader
			ctx.Error(http.StatusNotFound)
			return
		}
	} else { //If we have the repository we check access
		perm, err := models.GetUserRepoPermission(repository, ctx.User)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err.Error())
			return
		}
		if !perm.CanRead(unitType) {
			ctx.Error(http.StatusNotFound)
			return
		}
	}

	//If we have matched and access to release or issue
	fr, err := os.Open(attach.LocalPath())
	if err != nil {
		ctx.ServerError("Open", err)
		return
	}
	defer fr.Close()

	if err := attach.IncreaseDownloadCount(); err != nil {
		ctx.ServerError("Update", err)
		return
	}

	if err = ServeData(ctx, attach.Name, fr); err != nil {
		ctx.ServerError("ServeData", err)
		return
	}
}
