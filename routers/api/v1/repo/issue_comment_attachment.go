// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/attachment"
	comment_service "code.gitea.io/gitea/services/comments"
)

// GetIssueCommentAttachment gets a single attachment of the comment
func GetIssueCommentAttachment(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/comments/{id}/assets/{attachment_id} issue issueGetIssueCommentAttachment
	// ---
	// summary: Get a comment attachment
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
	//   description: id of the comment
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
	//     "$ref": "#/responses/error"

	comment := getIssueCommentSafe(ctx)
	if comment == nil {
		return
	}
	attach := getIssueCommentAttachmentSafeRead(ctx, comment)
	if attach == nil {
		return
	}
	if attach.CommentID != comment.ID {
		log.Debug("User requested attachment[%d] is not in comment[%d].", attach.ID, comment.ID)
		ctx.NotFound("attachment not in comment")
		return
	}

	ctx.JSON(http.StatusOK, convert.ToAttachment(attach))
}

// ListIssueCommentAttachments lists all attachments of the comment
func ListIssueCommentAttachments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/comments/{id}/assets issue issueListIssueCommentAttachments
	// ---
	// summary: List comment's attachments
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
	//   description: id of the comment
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/AttachmentList"
	//   "404":
	//     "$ref": "#/responses/error"
	comment := getIssueCommentSafe(ctx)
	if comment == nil {
		return
	}

	if err := comment.LoadAttachments(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttachments", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToComment(comment).Attachments)
}

// CreateIssueCommentAttachment creates an attachment and saves the given file
func CreateIssueCommentAttachment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/comments/{id}/assets issue issueCreateIssueCommentAttachment
	// ---
	// summary: Create a comment attachment
	// produces:
	// - application/json
	// consumes:
	// - multipart/form-data
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
	//   description: id of the comment
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
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/Attachment"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/error"

	// Check if comment exists and load comment
	comment := getIssueCommentSafe(ctx)
	if comment == nil {
		return
	}

	if !canUserWriteIssueCommentAttachment(ctx, comment) {
		return
	}

	// Get uploaded file from request
	file, header, err := ctx.Req.FormFile("attachment")
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetFile", err)
		return
	}
	defer file.Close()

	filename := header.Filename
	if query := ctx.FormString("name"); query != "" {
		filename = query
	}

	attach, err := attachment.UploadAttachment(file, setting.Attachment.AllowedTypes, &repo_model.Attachment{
		Name:       filename,
		UploaderID: ctx.Doer.ID,
		RepoID:     ctx.Repo.Repository.ID,
		IssueID:    comment.IssueID,
		CommentID:  comment.ID,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "UploadAttachment", err)
		return
	}
	if err := comment.LoadAttachments(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttachments", err)
		return
	}

	if err = comment_service.UpdateComment(comment, ctx.Doer, comment.Content); err != nil {
		ctx.ServerError("UpdateComment", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToAttachment(attach))
}

// EditIssueCommentAttachment updates the given attachment
func EditIssueCommentAttachment(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/issues/comments/{id}/assets/{attachment_id} issue issueEditIssueCommentAttachment
	// ---
	// summary: Edit a comment attachment
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
	//   description: id of the comment
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
	//   "404":
	//     "$ref": "#/responses/error"

	attach := getIssueCommentAttachmentSafeWrite(ctx)
	if attach == nil {
		return
	}

	form := web.GetForm(ctx).(*api.EditAttachmentOptions)
	if form.Name != "" {
		attach.Name = form.Name
	}

	if err := repo_model.UpdateAttachmentCtx(ctx, attach); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateAttachment", attach)
	}
	ctx.JSON(http.StatusCreated, convert.ToAttachment(attach))
}

// DeleteIssueCommentAttachment delete a given attachment
func DeleteIssueCommentAttachment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/comments/{id}/assets/{attachment_id} issue issueDeleteIssueCommentAttachment
	// ---
	// summary: Delete a comment attachment
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
	//   description: id of the comment
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
	//     "$ref": "#/responses/error"

	attach := getIssueCommentAttachmentSafeWrite(ctx)
	if attach == nil {
		return
	}

	if err := repo_model.DeleteAttachment(attach, true); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteAttachment", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func getIssueCommentSafe(ctx *context.APIContext) *models.Comment {
	comment, err := models.GetCommentByID(ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", models.IsErrCommentNotExist, err)
		return nil
	}
	// deny accessing arbitrary comments via this API
	// TODO: if issue ID were available on context, we could check that too.
	if err := comment.LoadIssue(); err != nil {
		ctx.Error(http.StatusInternalServerError, "comment.LoadIssue", err)
		return nil
	}
	if comment.Issue == nil || comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.Error(http.StatusNotFound, "", "no matching issue comment found")
		return nil
	}
	return comment
}

func getIssueCommentAttachmentSafeWrite(ctx *context.APIContext) *repo_model.Attachment {
	comment := getIssueCommentSafe(ctx)
	if comment == nil {
		return nil
	}
	if !canUserWriteIssueCommentAttachment(ctx, comment) {
		return nil
	}
	attach := getIssueCommentAttachmentSafeRead(ctx, comment)
	if attach == nil {
		return nil
	}
	return attach
}

func getIssueCommentAttachmentSafeRead(ctx *context.APIContext, comment *models.Comment) *repo_model.Attachment {
	attachID := ctx.ParamsInt64(":asset")
	attach, err := repo_model.GetAttachmentByID(attachID)
	if err != nil {
		ctx.NotFoundOrServerError("GetAttachmentByID", repo_model.IsErrAttachmentNotExist, err)
		return nil
	}
	if !attachmentBelongsToRepoOrComment(ctx, attach, comment) {
		return nil
	}
	return attach
}

func canUserWriteIssueCommentAttachment(ctx *context.APIContext, c *models.Comment) (success bool) {
	canEditComment := ctx.Doer.ID == c.PosterID || ctx.IsUserRepoAdmin() || ctx.IsUserSiteAdmin()
	if !canEditComment {
		ctx.Error(http.StatusForbidden, "", "user should have permission to edit comment")
		return
	}

	return true
}

func attachmentBelongsToRepoOrComment(ctx *context.APIContext, a *repo_model.Attachment, comment *models.Comment) (success bool) {
	if a.RepoID != ctx.Repo.Repository.ID {
		log.Debug("Requested attachment[%d] does not belong to repo[%-v].", a.ID, ctx.Repo.Repository)
		ctx.NotFound("no such attachment in repo")
		return
	}
	if a.IssueID == 0 || a.CommentID == 0 {
		// catch people trying to get release assets ;)
		log.Debug("Requested attachment[%d] is not in a comment.", a.ID)
		ctx.NotFound("no such attachment in comment")
		return
	}
	if comment != nil && a.CommentID != comment.ID {
		log.Debug("Requested attachment[%d] does not belong to comment[%d].", a.ID, comment.ID)
		ctx.NotFound("no such attachment in comment")
		return
	}
	return true
}
