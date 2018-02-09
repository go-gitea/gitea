// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// GetReleaseAttachment gets a single attachment of the release
func GetReleaseAttachment(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases/{id}/attachments/{attachment_id} repository repoGetReleaseAttachment
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
	//   required: true
	// - name: attachment_id
	//   in: path
	//   description: id of the attachment to get
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Attachment"
	releaseID := ctx.ParamsInt64(":id")
	attachID := ctx.ParamsInt64(":attachment")
	attach, err := models.GetAttachmentByID(attachID)
	if err != nil {
		ctx.Error(500, "GetAttachmentByID", err)
		return
	}
	if attach.ReleaseID != releaseID {
		ctx.Status(404)
		return
	}
	// FIXME Should prove the existence of the given repo, but results in unnecessary database requests
	ctx.JSON(200, attach.APIFormat())
}

// ListReleaseAttachments lists all attachments of the release
func ListReleaseAttachments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases/{id}/attachments repository repoListReleaseAttachments
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
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/AttachmentList"
	releaseID := ctx.ParamsInt64(":id")
	release, err := models.GetReleaseByID(releaseID)
	if err != nil {
		ctx.Error(500, "GetReleaseByID", err)
		return
	}
	if release.RepoID != ctx.Repo.Repository.ID {
		ctx.Status(404)
		return
	}
	if err := release.LoadAttributes(); err != nil {
		ctx.Error(500, "LoadAttributes", err)
		return
	}
	ctx.JSON(200, release.APIFormat().Attachments)
}
