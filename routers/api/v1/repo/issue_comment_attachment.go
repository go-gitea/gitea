// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	attachment_service "code.gitea.io/gitea/services/attachment"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/context/upload"
	"code.gitea.io/gitea/services/convert"
	issue_service "code.gitea.io/gitea/services/issue"
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
	attachment := getIssueCommentAttachmentSafeRead(ctx, comment)
	if attachment == nil {
		return
	}
	if attachment.CommentID != comment.ID {
		log.Debug("User requested attachment[%d] is not in comment[%d].", attachment.ID, comment.ID)
		ctx.APIErrorNotFound("attachment not in comment")
		return
	}

	ctx.JSON(http.StatusOK, convert.ToAPIAttachment(ctx.Repo.Repository, attachment))
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

	if err := comment.LoadAttachments(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToAPIAttachments(ctx.Repo.Repository, comment.Attachments))
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
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

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
		ctx.APIErrorInternal(err)
		return
	}
	defer file.Close()

	filename := header.Filename
	if query := ctx.FormString("name"); query != "" {
		filename = query
	}

	attachment, err := attachment_service.UploadAttachment(ctx, file, setting.Attachment.AllowedTypes, header.Size, &repo_model.Attachment{
		Name:       filename,
		UploaderID: ctx.Doer.ID,
		RepoID:     ctx.Repo.Repository.ID,
		IssueID:    comment.IssueID,
		CommentID:  comment.ID,
	})
	if err != nil {
		if upload.IsErrFileTypeForbidden(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if err := comment.LoadAttachments(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if err = issue_service.UpdateComment(ctx, comment, comment.ContentVersion, ctx.Doer, comment.Content); err != nil {
		if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.APIError(http.StatusForbidden, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToAPIAttachment(ctx.Repo.Repository, attachment))
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
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"
	attach := getIssueCommentAttachmentSafeWrite(ctx)
	if attach == nil {
		return
	}

	form := web.GetForm(ctx).(*api.EditAttachmentOptions)
	if form.Name != "" {
		attach.Name = form.Name
	}

	if err := attachment_service.UpdateAttachment(ctx, setting.Attachment.AllowedTypes, attach); err != nil {
		if upload.IsErrFileTypeForbidden(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err)
			return
		}
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToAPIAttachment(ctx.Repo.Repository, attach))
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
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"
	attach := getIssueCommentAttachmentSafeWrite(ctx)
	if attach == nil {
		return
	}

	if err := repo_model.DeleteAttachment(ctx, attach, true); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func getIssueCommentSafe(ctx *context.APIContext) *issues_model.Comment {
	comment, err := issues_model.GetCommentByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.NotFoundOrServerError(err)
		return nil
	}
	if err := comment.LoadIssue(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return nil
	}
	if comment.Issue == nil || comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.APIError(http.StatusNotFound, "no matching issue comment found")
		return nil
	}

	if !ctx.Repo.CanReadIssuesOrPulls(comment.Issue.IsPull) {
		return nil
	}

	comment.Issue.Repo = ctx.Repo.Repository

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
	return getIssueCommentAttachmentSafeRead(ctx, comment)
}

func canUserWriteIssueCommentAttachment(ctx *context.APIContext, comment *issues_model.Comment) bool {
	canEditComment := ctx.IsSigned && (ctx.Doer.ID == comment.PosterID || ctx.IsUserRepoAdmin() || ctx.IsUserSiteAdmin()) && ctx.Repo.CanWriteIssuesOrPulls(comment.Issue.IsPull)
	if !canEditComment {
		ctx.APIError(http.StatusForbidden, "user should have permission to edit comment")
		return false
	}

	return true
}

func getIssueCommentAttachmentSafeRead(ctx *context.APIContext, comment *issues_model.Comment) *repo_model.Attachment {
	attachment, err := repo_model.GetAttachmentByID(ctx, ctx.PathParamInt64("attachment_id"))
	if err != nil {
		ctx.NotFoundOrServerError(err)
		return nil
	}
	if !attachmentBelongsToRepoOrComment(ctx, attachment, comment) {
		return nil
	}
	return attachment
}

func attachmentBelongsToRepoOrComment(ctx *context.APIContext, attachment *repo_model.Attachment, comment *issues_model.Comment) bool {
	if attachment.RepoID != ctx.Repo.Repository.ID {
		log.Debug("Requested attachment[%d] does not belong to repo[%-v].", attachment.ID, ctx.Repo.Repository)
		ctx.APIErrorNotFound("no such attachment in repo")
		return false
	}
	if attachment.IssueID == 0 || attachment.CommentID == 0 {
		log.Debug("Requested attachment[%d] is not in a comment.", attachment.ID)
		ctx.APIErrorNotFound("no such attachment in comment")
		return false
	}
	if comment != nil && attachment.CommentID != comment.ID {
		log.Debug("Requested attachment[%d] does not belong to comment[%d].", attachment.ID, comment.ID)
		ctx.APIErrorNotFound("no such attachment in comment")
		return false
	}
	return true
}
