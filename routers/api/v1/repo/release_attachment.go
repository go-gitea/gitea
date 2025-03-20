// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"io"
	"net/http"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	attachment_service "code.gitea.io/gitea/services/attachment"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/context/upload"
	"code.gitea.io/gitea/services/convert"
)

func checkReleaseMatchRepo(ctx *context.APIContext, releaseID int64) bool {
	release, err := repo_model.GetReleaseByID(ctx, releaseID)
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
			ctx.APIErrorNotFound()
			return false
		}
		ctx.APIErrorInternal(err)
		return false
	}
	if release.RepoID != ctx.Repo.Repository.ID {
		ctx.APIErrorNotFound()
		return false
	}
	return true
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
	if !checkReleaseMatchRepo(ctx, releaseID) {
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

	releaseID := ctx.PathParamInt64("id")
	release, err := repo_model.GetReleaseByID(ctx, releaseID)
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
			ctx.APIErrorNotFound()
			return
		}
		ctx.APIErrorInternal(err)
		return
	}
	if release.RepoID != ctx.Repo.Repository.ID {
		ctx.APIErrorNotFound()
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

	// Check if attachments are enabled
	if !setting.Attachment.Enabled {
		ctx.APIErrorNotFound("Attachment is not enabled")
		return
	}

	// Check if release exists an load release
	releaseID := ctx.PathParamInt64("id")
	if !checkReleaseMatchRepo(ctx, releaseID) {
		return
	}

	// Get uploaded file from request
	var content io.ReadCloser
	var filename string
	var size int64 = -1

	if strings.HasPrefix(strings.ToLower(ctx.Req.Header.Get("Content-Type")), "multipart/form-data") {
		file, header, err := ctx.Req.FormFile("attachment")
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		defer file.Close()

		content = file
		size = header.Size
		filename = header.Filename
		if name := ctx.FormString("name"); name != "" {
			filename = name
		}
	} else {
		content = ctx.Req.Body
		filename = ctx.FormString("name")
	}

	if filename == "" {
		ctx.APIError(http.StatusBadRequest, "Could not determine name of attachment.")
		return
	}

	// Create a new attachment and save the file
	attach, err := attachment_service.UploadAttachment(ctx, content, setting.Repository.Release.AllowedTypes, size, &repo_model.Attachment{
		Name:       filename,
		UploaderID: ctx.Doer.ID,
		RepoID:     ctx.Repo.Repository.ID,
		ReleaseID:  releaseID,
	})
	if err != nil {
		if upload.IsErrFileTypeForbidden(err) {
			ctx.APIError(http.StatusBadRequest, err)
			return
		}
		ctx.APIErrorInternal(err)
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

	form := web.GetForm(ctx).(*api.EditAttachmentOptions)

	// Check if release exists an load release
	releaseID := ctx.PathParamInt64("id")
	if !checkReleaseMatchRepo(ctx, releaseID) {
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
			ctx.APIError(http.StatusUnprocessableEntity, err)
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
	if !checkReleaseMatchRepo(ctx, releaseID) {
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

	if err := repo_model.DeleteAttachment(ctx, attach, true); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
