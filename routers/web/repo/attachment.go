// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/upload"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/services/attachment"
)

// UploadIssueAttachment response for Issue/PR attachments
func UploadIssueAttachment(ctx *context.Context) {
	uploadAttachment(ctx, ctx.Repo.Repository.ID, setting.Attachment.AllowedTypes)
}

// UploadReleaseAttachment response for uploading release attachments
func UploadReleaseAttachment(ctx *context.Context) {
	uploadAttachment(ctx, ctx.Repo.Repository.ID, setting.Repository.Release.AllowedTypes)
}

// UploadAttachment response for uploading attachments
func uploadAttachment(ctx *context.Context, repoID int64, allowedTypes string) {
	if !setting.Attachment.Enabled {
		ctx.Error(http.StatusNotFound, "attachment is not enabled")
		return
	}

	file, header, err := ctx.Req.FormFile("file")
	if err != nil {
		ctx.Error(http.StatusInternalServerError, fmt.Sprintf("FormFile: %v", err))
		return
	}
	defer file.Close()

	attach, err := attachment.UploadAttachment(file, ctx.User.ID, repoID, 0, header.Filename, allowedTypes)
	if err != nil {
		if upload.IsErrFileTypeForbidden(err) {
			ctx.Error(http.StatusBadRequest, err.Error())
			return
		}
		ctx.Error(http.StatusInternalServerError, fmt.Sprintf("NewAttachment: %v", err))
		return
	}

	log.Trace("New attachment uploaded: %s", attach.UUID)
	ctx.JSON(http.StatusOK, map[string]string{
		"uuid": attach.UUID,
	})
}

// DeleteAttachment response for deleting issue's attachment
func DeleteAttachment(ctx *context.Context) {
	file := ctx.FormString("file")
	attach, err := repo_model.GetAttachmentByUUID(file)
	if err != nil {
		ctx.Error(http.StatusBadRequest, err.Error())
		return
	}
	if !ctx.IsSigned || (ctx.User.ID != attach.UploaderID) {
		ctx.Error(http.StatusForbidden)
		return
	}
	err = repo_model.DeleteAttachment(attach, true)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, fmt.Sprintf("DeleteAttachment: %v", err))
		return
	}
	ctx.JSON(http.StatusOK, map[string]string{
		"uuid": attach.UUID,
	})
}

// GetAttachment serve attachements
func GetAttachment(ctx *context.Context) {
	attach, err := repo_model.GetAttachmentByUUID(ctx.Params(":uuid"))
	if err != nil {
		if repo_model.IsErrAttachmentNotExist(err) {
			ctx.Error(http.StatusNotFound)
		} else {
			ctx.ServerError("GetAttachmentByUUID", err)
		}
		return
	}

	repository, unitType, err := models.LinkedRepository(attach)
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

	if err := attach.IncreaseDownloadCount(); err != nil {
		ctx.ServerError("IncreaseDownloadCount", err)
		return
	}

	if setting.Attachment.ServeDirect {
		//If we have a signed url (S3, object storage), redirect to this directly.
		u, err := storage.Attachments.URL(attach.RelativePath(), attach.Name)

		if u != nil && err == nil {
			ctx.Redirect(u.String())
			return
		}
	}

	if httpcache.HandleGenericETagCache(ctx.Req, ctx.Resp, `"`+attach.UUID+`"`) {
		return
	}

	//If we have matched and access to release or issue
	fr, err := storage.Attachments.Open(attach.RelativePath())
	if err != nil {
		ctx.ServerError("Open", err)
		return
	}
	defer fr.Close()

	if err = common.ServeData(ctx, attach.Name, attach.Size, fr); err != nil {
		ctx.ServerError("ServeData", err)
		return
	}
}
