// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
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
		ctx.ServerError("FormFile", err)
		return
	}
	defer file.Close()

	uploaderFile := attachment.NewLimitedUploaderKnownSize(file, header.Size)
	attach, err := attachment.UploadAttachmentGeneralSizeLimit(ctx, uploaderFile, allowedTypes, &repo_model.Attachment{
		Name:       header.Filename,
		UploaderID: ctx.Doer.ID,
		RepoID:     repoID,
	})
	if err != nil {
		if upload.IsErrFileTypeForbidden(err) {
			ctx.HTTPError(http.StatusBadRequest, err.Error())
			return
		}
		ctx.ServerError("UploadAttachmentGeneralSizeLimit", err)
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

	if !ctx.IsSigned {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	if attach.RepoID != ctx.Repo.Repository.ID {
		ctx.HTTPError(http.StatusBadRequest, "attachment does not belong to this repository")
		return
	}

	if ctx.Doer.ID != attach.UploaderID {
		if attach.IssueID > 0 {
			issue, err := issues_model.GetIssueByID(ctx, attach.IssueID)
			if err != nil {
				ctx.ServerError("GetIssueByID", err)
				return
			}
			if !ctx.Repo.Permission.CanWriteIssuesOrPulls(issue.IsPull) {
				ctx.HTTPError(http.StatusForbidden)
				return
			}
		} else if attach.ReleaseID > 0 {
			if !ctx.Repo.Permission.CanWrite(unit.TypeReleases) {
				ctx.HTTPError(http.StatusForbidden)
				return
			}
		} else {
			if !ctx.Repo.Permission.IsAdmin() && !ctx.Repo.Permission.IsOwner() {
				ctx.HTTPError(http.StatusForbidden)
				return
			}
		}
	}

	err = repo_model.DeleteAttachment(ctx, attach, true)
	if err != nil {
		ctx.ServerError("DeleteAttachment", err)
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

	// prevent visiting attachment from other repository directly
	// The check will be ignored before this code merged.
	if attach.CreatedUnix > repo_model.LegacyAttachmentMissingRepoIDCutoff && ctx.Repo.Repository != nil && ctx.Repo.Repository.ID != attach.RepoID {
		ctx.HTTPError(http.StatusNotFound)
		return
	}

	unitType, repoID, err := repo_service.GetAttachmentLinkedTypeAndRepoID(ctx, attach)
	if err != nil {
		ctx.ServerError("GetAttachmentLinkedTypeAndRepoID", err)
		return
	}

	if unitType == unit.TypeInvalid { // unlinked attachment can only be accessed by the uploader
		if !(ctx.IsSigned && attach.UploaderID == ctx.Doer.ID) { // We block if not the uploader
			ctx.HTTPError(http.StatusNotFound)
			return
		}
	} else { // If we have the linked type, we need to check access
		var perm access_model.Permission
		if ctx.Repo.Repository == nil {
			repo, err := repo_model.GetRepositoryByID(ctx, repoID)
			if err != nil {
				ctx.ServerError("GetRepositoryByID", err)
				return
			}
			perm, err = access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
			if err != nil {
				ctx.ServerError("GetUserRepoPermission", err)
				return
			}
		} else {
			perm = ctx.Repo.Permission
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
		u, err := storage.Attachments.URL(attach.RelativePath(), attach.Name, ctx.Req.Method, nil)

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

	common.ServeContentByReadSeeker(ctx.Base, attach.Name, new(attach.CreatedUnix.AsTime()), fr)
}

// GetAttachment serve attachments
func GetAttachment(ctx *context.Context) {
	ServeAttachment(ctx, ctx.PathParam("uuid"))
}
