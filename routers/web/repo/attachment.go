// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"

	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/services/attachment"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/context/upload"
	repo_service "code.gitea.io/gitea/services/repository"
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
		ctx.HTTPError(http.StatusNotFound, "attachment is not enabled")
		return
	}

	file, header, err := ctx.Req.FormFile("file")
	if err != nil {
		ctx.HTTPError(http.StatusInternalServerError, fmt.Sprintf("FormFile: %v", err))
		return
	}
	defer file.Close()

	attach, err := attachment.UploadAttachment(ctx, file, allowedTypes, header.Size, &repo_model.Attachment{
		Name:       header.Filename,
		UploaderID: ctx.Doer.ID,
		RepoID:     repoID,
	})
	if err != nil {
		if upload.IsErrFileTypeForbidden(err) {
			ctx.HTTPError(http.StatusBadRequest, err.Error())
			return
		}
		ctx.HTTPError(http.StatusInternalServerError, fmt.Sprintf("NewAttachment: %v", err))
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
	attach, err := repo_model.GetAttachmentByUUID(ctx, file)
	if err != nil {
		ctx.HTTPError(http.StatusBadRequest, err.Error())
		return
	}
	if !ctx.IsSigned || (ctx.Doer.ID != attach.UploaderID) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}
	err = repo_model.DeleteAttachment(ctx, attach, true)
	if err != nil {
		ctx.HTTPError(http.StatusInternalServerError, fmt.Sprintf("DeleteAttachment: %v", err))
		return
	}
	ctx.JSON(http.StatusOK, map[string]string{
		"uuid": attach.UUID,
	})
}

// GetAttachment serve attachments with the given UUID
func ServeAttachment(ctx *context.Context, uuid string) {
	attach, err := repo_model.GetAttachmentByUUID(ctx, uuid)
	if err != nil {
		if repo_model.IsErrAttachmentNotExist(err) {
			ctx.HTTPError(http.StatusNotFound)
		} else {
			ctx.ServerError("GetAttachmentByUUID", err)
		}
		return
	}

	repository, unitType, err := repo_service.LinkedRepository(ctx, attach)
	if err != nil {
		ctx.ServerError("LinkedRepository", err)
		return
	}

	if repository == nil { // If not linked
		if !(ctx.IsSigned && attach.UploaderID == ctx.Doer.ID) { // We block if not the uploader
			ctx.HTTPError(http.StatusNotFound)
			return
		}
	} else { // If we have the repository we check access
		perm, err := access_model.GetUserRepoPermission(ctx, repository, ctx.Doer)
		if err != nil {
			ctx.HTTPError(http.StatusInternalServerError, "GetUserRepoPermission", err.Error())
			return
		}
		if !perm.CanRead(unitType) {
			ctx.HTTPError(http.StatusNotFound)
			return
		}
	}

	if err := attach.IncreaseDownloadCount(ctx); err != nil {
		ctx.ServerError("IncreaseDownloadCount", err)
		return
	}

	if setting.Attachment.Storage.ServeDirect() {
		// If we have a signed url (S3, object storage), redirect to this directly.
		u, err := storage.Attachments.URL(attach.RelativePath(), attach.Name, nil)

		if u != nil && err == nil {
			ctx.Redirect(u.String())
			return
		}
	}

	if httpcache.HandleGenericETagCache(ctx.Req, ctx.Resp, `"`+attach.UUID+`"`) {
		return
	}

	// If we have matched and access to release or issue
	fr, err := storage.Attachments.Open(attach.RelativePath())
	if err != nil {
		ctx.ServerError("Open", err)
		return
	}
	defer fr.Close()

	common.ServeContentByReadSeeker(ctx.Base, attach.Name, util.ToPointer(attach.CreatedUnix.AsTime()), fr)
}

// GetAttachment serve attachments
func GetAttachment(ctx *context.Context) {
	ServeAttachment(ctx, ctx.PathParam("uuid"))
}
