// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	attachment_service "gitea.dev/services/attachment"
	"gitea.dev/services/context"
	"gitea.dev/services/context/upload"
	"gitea.dev/services/convert"
	release_service "gitea.dev/services/release"
)

func checkReleaseAssetsMutable(ctx *context.APIContext, releaseID int64) bool {
	release := checkReleaseMatchRepo(ctx, releaseID)
	if release == nil {
		return false
	}
	if release.IsImmutable {
		ctx.APIErrorAuto(release_service.ErrImmutableRelease)
		return false
	}
	return true
}

// checkReleaseMatchRepo returns nil once it has written the response itself.
func checkReleaseMatchRepo(ctx *context.APIContext, releaseID int64) *repo_model.Release {
	release, err := repo_model.GetReleaseForRepoByID(ctx, ctx.Repo.Repository.ID, releaseID)
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return nil
	}
	if release.IsDraft && !canAccessReleaseDraft(ctx) {
		ctx.APIErrorNotFound()
		return nil
	}
	return release
}

// GetReleaseAttachment gets a single attachment of the release
func GetReleaseAttachment(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases/{id}/assets/{attachment_id} repository repoGetReleaseAttachment
	// ---
	// summary: Get a release attachment
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
	// - name: id
	//   in: path
	//   description: id of the release
	//   type: integer
	//   format: int64
	//   required: true
	// - name: attachment_id
	//   in: path
	//   description: id of the attachment to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Attachment"
	//   "404":
	//     "$ref": "#/responses/notFound"

	releaseID := ctx.PathParamInt64("id")
	if checkReleaseMatchRepo(ctx, releaseID) == nil {
		return
	}

	attachID := ctx.PathParamInt64("attachment_id")
	attach, err := repo_model.GetAttachmentByID(ctx, attachID)
	if err != nil {
		if repo_model.IsErrAttachmentNotExist(err) {
			ctx.APIErrorNotFound()
			return
		}
		ctx.APIErrorInternal(err)
		return
	}
	if attach.ReleaseID != releaseID {
		log.Info("User requested attachment is not in release, release_id %v, attachment_id: %v", releaseID, attachID)
		ctx.APIErrorNotFound()
		return
	}
	// FIXME Should prove the existence of the given repo, but results in unnecessary database requests
	ctx.JSON(http.StatusOK, convert.ToAPIAttachment(ctx.Repo.Repository, attach))
}

// ListReleaseAttachments lists all attachments of the release
func ListReleaseAttachments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases/{id}/assets repository repoListReleaseAttachments
	// ---
	// summary: List release's attachments
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
	// - name: id
	//   in: path
	//   description: id of the release
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/AttachmentList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	release := checkReleaseMatchRepo(ctx, ctx.PathParamInt64("id"))
	if release == nil {
		return
	}
	if err := release.LoadAttributes(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToAPIRelease(ctx, ctx.Repo.Repository, release).Attachments)
}

// CreateReleaseAttachment creates an attachment and saves the given file
func CreateReleaseAttachment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/releases/{id}/assets repository repoCreateReleaseAttachment
	// ---
	// summary: Create a release attachment
	// produces:
	// - application/json
	// consumes:
	// - multipart/form-data
	// - application/octet-stream
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
	// - name: id
	//   in: path
	//   description: id of the release
	//   type: integer
	//   format: int64
	//   required: true
	// - name: name
	//   in: query
	//   description: name of the attachment
	//   type: string
	//   required: false
	// - name: attachment
	//   in: formData
	//   description: attachment to upload
	//   type: file
	//   required: false
	// responses:
	//   "201":
	//     "$ref": "#/responses/Attachment"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "413":
	//     "$ref": "#/responses/error"

	// Check if attachments are enabled
	if !setting.Attachment.Enabled {
		ctx.APIErrorNotFound("attachment is not enabled")
		return
	}

	// Check if release exists an load release
	releaseID := ctx.PathParamInt64("id")
	if !checkReleaseAssetsMutable(ctx, releaseID) {
		return
	}

	// Get uploaded file from request
	var filename string
	var uploaderFile *attachment_service.UploaderFile
	if strings.HasPrefix(strings.ToLower(ctx.Req.Header.Get("Content-Type")), "multipart/form-data") {
		file, header, err := ctx.Req.FormFile("attachment")
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		defer file.Close()

		filename = header.Filename
		if name := ctx.FormString("name"); name != "" {
			filename = name
		}
		uploaderFile = attachment_service.NewLimitedUploaderKnownSize(file, header.Size)
	} else {
		filename = ctx.FormString("name")
		uploaderFile = attachment_service.NewLimitedUploaderMaxBytesReader(ctx.Req.Body, ctx.Resp)
	}

	if filename == "" {
		ctx.APIError(http.StatusBadRequest, "Could not determine name of attachment.")
		return
	}

	// Create a new attachment and save the file
	attach, err := attachment_service.UploadAttachmentForRelease(ctx, uploaderFile, &repo_model.Attachment{
		Name:       filename,
		UploaderID: ctx.Doer.ID,
		RepoID:     ctx.Repo.Repository.ID,
		ReleaseID:  releaseID,
	})
	if err != nil {
		if upload.IsErrFileTypeForbidden(err) {
			ctx.APIError(http.StatusBadRequest, err.Error())
			return
		}

		if errors.Is(err, util.ErrContentTooLarge) {
			ctx.APIError(http.StatusRequestEntityTooLarge, err.Error())
			return
		}

		ctx.APIErrorInternal(err)
		return
	}

	// the release may have been published while the upload was streaming, but only an actual lock
	// may discard what was uploaded, a transient failure must not
	release := checkReleaseMatchRepo(ctx, releaseID)
	if release == nil {
		return
	}
	if release.IsImmutable {
		if err := repo_model.DeleteAttachment(ctx, attach, true); err != nil {
			log.Error("DeleteAttachment %s: %v", attach.UUID, err)
		}
		ctx.APIErrorAuto(release_service.ErrImmutableRelease)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToAPIAttachment(ctx.Repo.Repository, attach))
}

// EditReleaseAttachment updates the given attachment
func EditReleaseAttachment(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/releases/{id}/assets/{attachment_id} repository repoEditReleaseAttachment
	// ---
	// summary: Edit a release attachment
	// produces:
	// - application/json
	// consumes:
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
	// - name: id
	//   in: path
	//   description: id of the release
	//   type: integer
	//   format: int64
	//   required: true
	// - name: attachment_id
	//   in: path
	//   description: id of the attachment to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditAttachmentOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Attachment"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	form := web.GetForm[*api.EditAttachmentOptions](ctx)

	// Check if release exists an load release
	releaseID := ctx.PathParamInt64("id")
	if !checkReleaseAssetsMutable(ctx, releaseID) {
		return
	}

	attachID := ctx.PathParamInt64("attachment_id")
	attach, err := repo_model.GetAttachmentByID(ctx, attachID)
	if err != nil {
		if repo_model.IsErrAttachmentNotExist(err) {
			ctx.APIErrorNotFound()
			return
		}
		ctx.APIErrorInternal(err)
		return
	}
	if attach.ReleaseID != releaseID {
		log.Info("User requested attachment is not in release, release_id %v, attachment_id: %v", releaseID, attachID)
		ctx.APIErrorNotFound()
		return
	}
	// FIXME Should prove the existence of the given repo, but results in unnecessary database requests
	if form.Name != "" {
		attach.Name = form.Name
	}

	if err := attachment_service.UpdateAttachment(ctx, setting.Repository.Release.AllowedTypes, attach); err != nil {
		if upload.IsErrFileTypeForbidden(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err.Error())
			return
		}
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToAPIAttachment(ctx.Repo.Repository, attach))
}

// DeleteReleaseAttachment delete a given attachment
func DeleteReleaseAttachment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/releases/{id}/assets/{attachment_id} repository repoDeleteReleaseAttachment
	// ---
	// summary: Delete a release attachment
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
	// - name: id
	//   in: path
	//   description: id of the release
	//   type: integer
	//   format: int64
	//   required: true
	// - name: attachment_id
	//   in: path
	//   description: id of the attachment to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// Check if release exists an load release
	releaseID := ctx.PathParamInt64("id")
	if !checkReleaseAssetsMutable(ctx, releaseID) {
		return
	}

	attachID := ctx.PathParamInt64("attachment_id")
	attach, err := repo_model.GetAttachmentByID(ctx, attachID)
	if err != nil {
		if repo_model.IsErrAttachmentNotExist(err) {
			ctx.APIErrorNotFound()
			return
		}
		ctx.APIErrorInternal(err)
		return
	}
	if attach.ReleaseID != releaseID {
		log.Info("User requested attachment is not in release, release_id %v, attachment_id: %v", releaseID, attachID)
		ctx.APIErrorNotFound()
		return
	}

	if err := repo_model.DeleteAttachment(ctx, attach, true); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
