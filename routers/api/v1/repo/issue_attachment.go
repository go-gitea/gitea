// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/attachment"
	"code.gitea.io/gitea/services/convert"
	issue_service "code.gitea.io/gitea/services/issue"
)

// GetIssueAttachment gets a single attachment of the issue
func GetIssueAttachment(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{index}/assets/{attachment_id} issue issueGetIssueAttachment
	// ---
	// summary: Get an issue attachment
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
	// - name: index
	//   in: path
	//   description: index of the issue
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

	issue := getIssueFromContext(ctx)
	if issue == nil {
		return
	}

	attach := getIssueAttachmentSafeRead(ctx, issue)
	if attach == nil {
		return
	}

	ctx.JSON(http.StatusOK, convert.ToAPIAttachment(ctx.Repo.Repository, attach))
}

// ListIssueAttachments lists all attachments of the issue
func ListIssueAttachments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{index}/assets issue issueListIssueAttachments
	// ---
	// summary: List issue's attachments
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
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/AttachmentList"
	//   "404":
	//     "$ref": "#/responses/error"

	issue := getIssueFromContext(ctx)
	if issue == nil {
		return
	}

	if err := issue.LoadAttributes(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToAPIIssue(ctx, issue).Attachments)
}

// CreateIssueAttachment creates an attachment and saves the given file
func CreateIssueAttachment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/assets issue issueCreateIssueAttachment
	// ---
	// summary: Create an issue attachment
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
	// - name: index
	//   in: path
	//   description: index of the issue
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
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	issue := getIssueFromContext(ctx)
	if issue == nil {
		return
	}

	if !canUserWriteIssueAttachment(ctx, issue) {
		return
	}

	// Get uploaded file from request
	file, header, err := ctx.Req.FormFile("attachment")
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FormFile", err)
		return
	}
	defer file.Close()

	filename := header.Filename
	if query := ctx.FormString("name"); query != "" {
		filename = query
	}

	attachment, err := attachment.UploadAttachment(ctx, file, setting.Attachment.AllowedTypes, header.Size, &repo_model.Attachment{
		Name:       filename,
		UploaderID: ctx.Doer.ID,
		RepoID:     ctx.Repo.Repository.ID,
		IssueID:    issue.ID,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "UploadAttachment", err)
		return
	}

	issue.Attachments = append(issue.Attachments, attachment)

	if err := issue_service.ChangeContent(ctx, issue, ctx.Doer, issue.Content); err != nil {
		ctx.Error(http.StatusInternalServerError, "ChangeContent", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToAPIAttachment(ctx.Repo.Repository, attachment))
}

// EditIssueAttachment updates the given attachment
func EditIssueAttachment(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/issues/{index}/assets/{attachment_id} issue issueEditIssueAttachment
	// ---
	// summary: Edit an issue attachment
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
	// - name: index
	//   in: path
	//   description: index of the issue
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
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	attachment := getIssueAttachmentSafeWrite(ctx)
	if attachment == nil {
		return
	}

	// do changes to attachment. only meaningful change is name.
	form := web.GetForm(ctx).(*api.EditAttachmentOptions)
	if form.Name != "" {
		attachment.Name = form.Name
	}

	if err := repo_model.UpdateAttachment(ctx, attachment); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateAttachment", err)
	}

	ctx.JSON(http.StatusCreated, convert.ToAPIAttachment(ctx.Repo.Repository, attachment))
}

// DeleteIssueAttachment delete a given attachment
func DeleteIssueAttachment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/{index}/assets/{attachment_id} issue issueDeleteIssueAttachment
	// ---
	// summary: Delete an issue attachment
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
	// - name: index
	//   in: path
	//   description: index of the issue
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

	attachment := getIssueAttachmentSafeWrite(ctx)
	if attachment == nil {
		return
	}

	if err := repo_model.DeleteAttachment(ctx, attachment, true); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteAttachment", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func getIssueFromContext(ctx *context.APIContext) *issues_model.Issue {
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.ParamsInt64("index"))
	if err != nil {
		ctx.NotFoundOrServerError("GetIssueByIndex", issues_model.IsErrIssueNotExist, err)
		return nil
	}

	issue.Repo = ctx.Repo.Repository

	return issue
}

func getIssueAttachmentSafeWrite(ctx *context.APIContext) *repo_model.Attachment {
	issue := getIssueFromContext(ctx)
	if issue == nil {
		return nil
	}

	if !canUserWriteIssueAttachment(ctx, issue) {
		return nil
	}

	return getIssueAttachmentSafeRead(ctx, issue)
}

func getIssueAttachmentSafeRead(ctx *context.APIContext, issue *issues_model.Issue) *repo_model.Attachment {
	attachment, err := repo_model.GetAttachmentByID(ctx, ctx.ParamsInt64("attachment_id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetAttachmentByID", repo_model.IsErrAttachmentNotExist, err)
		return nil
	}
	if !attachmentBelongsToRepoOrIssue(ctx, attachment, issue) {
		return nil
	}
	return attachment
}

func canUserWriteIssueAttachment(ctx *context.APIContext, issue *issues_model.Issue) bool {
	canEditIssue := ctx.IsSigned && (ctx.Doer.ID == issue.PosterID || ctx.IsUserRepoAdmin() || ctx.IsUserSiteAdmin() || ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull))
	if !canEditIssue {
		ctx.Error(http.StatusForbidden, "", "user should have permission to write issue")
		return false
	}

	return true
}

func attachmentBelongsToRepoOrIssue(ctx *context.APIContext, attachment *repo_model.Attachment, issue *issues_model.Issue) bool {
	if attachment.RepoID != ctx.Repo.Repository.ID {
		log.Debug("Requested attachment[%d] does not belong to repo[%-v].", attachment.ID, ctx.Repo.Repository)
		ctx.NotFound("no such attachment in repo")
		return false
	}
	if attachment.IssueID == 0 {
		log.Debug("Requested attachment[%d] is not in an issue.", attachment.ID)
		ctx.NotFound("no such attachment in issue")
		return false
	} else if issue != nil && attachment.IssueID != issue.ID {
		log.Debug("Requested attachment[%d] does not belong to issue[%d, #%d].", attachment.ID, issue.ID, issue.Index)
		ctx.NotFound("no such attachment in issue")
		return false
	}
	return true
}
