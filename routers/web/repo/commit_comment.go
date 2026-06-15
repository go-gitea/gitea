// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/context/upload"
	"gitea.dev/services/forms"
	repo_service "gitea.dev/services/repository"
)

const (
	tplCommitNewComment   templates.TplName = "repo/diff/commit_new_comment"
	tplCommitConversation templates.TplName = "repo/diff/commit_conversation"
)

// RenderNewCommitCommentForm renders the form for creating a new inline comment on a commit.
func RenderNewCommitCommentForm(ctx *context.Context) {
	ctx.Data["PageIsCommitDiff"] = true
	ctx.Data["CommitID"] = ctx.PathParam("sha")
	ctx.Data["AfterCommitID"] = ctx.PathParam("sha")
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")
	ctx.HTML(http.StatusOK, tplCommitNewComment)
}

// CreateCommitComment creates an inline comment on a commit and renders the updated conversation.
func CreateCommitComment(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CommitCommentForm)
	commitID := ctx.PathParam("sha")

	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	signedLine := form.Line
	if form.Side == "previous" {
		signedLine *= -1
	}

	var attachments []string
	if setting.Attachment.Enabled {
		attachments = form.Files
	}

	comment, err := repo_service.CreateCommitComment(ctx,
		ctx.Doer,
		ctx.Repo.Repository,
		ctx.Repo.GitRepo,
		commitID,
		form.Content,
		form.TreePath,
		signedLine,
		attachments,
	)
	if err != nil {
		ctx.ServerError("CreateCommitComment", err)
		return
	}

	log.Trace("Commit comment created: %-v Commit[%s] Comment[%d]", ctx.Repo.Repository, comment.CommitSHA, comment.ID)

	renderCommitConversation(ctx, comment.CommitSHA, comment.TreePath, comment.Line)
}

// renderCommitConversation renders all inline comments at a given path/line of a commit.
func renderCommitConversation(ctx *context.Context, commitSHA, treePath string, line int64) {
	allComments, err := issues_model.FetchCommitCodeComments(ctx, ctx.Repo.Repository, commitSHA, ctx.Doer)
	if err != nil {
		ctx.ServerError("FetchCommitCodeComments", err)
		return
	}
	comments := allComments[treePath][line]
	if len(comments) == 0 {
		ctx.HTML(http.StatusOK, tplConversationOutdated)
		return
	}

	ctx.Data["PageIsCommitDiff"] = true
	ctx.Data["CommitID"] = commitSHA
	ctx.Data["AfterCommitID"] = commitSHA
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")
	ctx.Data["comments"] = comments
	ctx.HTML(http.StatusOK, tplCommitConversation)
}
