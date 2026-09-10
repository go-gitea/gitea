// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"strconv"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/renderhelper"
	unit_model "gitea.dev/models/unit"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
	"gitea.dev/services/context/upload"
	repo_service "gitea.dev/services/repository"
)

var (
	tplNewCommitComment   templates.TplName = "repo/diff/new_commit_comment"
	tplCommitConversation templates.TplName = "repo/diff/commit_conversation"
)

func canCommentOnCommit(ctx *context.Context) bool {
	return ctx.Doer != nil && !ctx.Repo.Repository.IsArchived && ctx.Repo.Permission.CanRead(unit_model.TypeCode)
}

func commitCommentURL(ctx *context.Context, commitSHA string) string {
	return ctx.Repo.RepoLink + "/commit/" + commitSHA + "/comment"
}

func renderCommitComments(ctx *context.Context, comments issues_model.CommentList) {
	for _, c := range comments {
		rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository, renderhelper.RepoCommentOptions{
			FootnoteContextID: strconv.FormatInt(c.ID, 10),
		})
		var err error
		if c.RenderedContent, err = markdown.RenderString(rctx, c.Content); err != nil {
			log.Error("RenderString for commit comment %d: %v", c.ID, err)
		}
	}
}

// RenderNewCommitCommentForm renders the form used by the diff "+" button.
func RenderNewCommitCommentForm(ctx *context.Context) {
	if !canCommentOnCommit(ctx) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}
	commitSHA := ctx.PathParam("sha")
	ctx.Data["CanCommentOnCommit"] = true
	ctx.Data["DiffNewCommentURL"] = commitCommentURL(ctx, commitSHA)
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")
	ctx.HTML(http.StatusOK, tplNewCommitComment)
}

// CreateCommitComment creates an inline comment on a commit diff.
func CreateCommitComment(ctx *context.Context) {
	if !canCommentOnCommit(ctx) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	commitSHA := ctx.PathParam("sha")
	if commitSHA == "" {
		ctx.NotFound(nil)
		return
	}

	content := ctx.FormString("content")
	treePath := ctx.FormString("path")
	side := ctx.FormString("side")
	line := ctx.FormInt64("line")
	attachments := ctx.FormStrings("files")

	if content == "" || treePath == "" || line <= 0 {
		ctx.JSONError("content, path, and a positive line are required")
		return
	}
	if side != "previous" && side != "proposed" {
		ctx.JSONError("side must be either 'previous' or 'proposed'")
		return
	}
	if side == "previous" {
		line = -line
	}

	comment, err := repo_service.CreateCommitComment(ctx, ctx.Doer, ctx.Repo.Repository, ctx.Repo.GitRepo, commitSHA, treePath, line, content, attachments)
	switch {
	case err == nil:
	case git.IsErrNotExist(err):
		ctx.NotFound(err)
		return
	case errors.Is(err, repo_service.ErrCommitCommentCoordinates), errors.Is(err, repo_service.ErrCommitCommentRootPrevious):
		ctx.JSONError(err.Error())
		return
	default:
		ctx.ServerError("CreateCommitComment", err)
		return
	}

	comments, err := issues_model.FindCommitCommentsByLine(ctx, ctx.Repo.Repository.ID, comment.CommitSHA, treePath, line)
	if err != nil {
		ctx.ServerError("FindCommitCommentsByLine", err)
		return
	}
	renderCommitComments(ctx, comments)

	ctx.Data["CanCommentOnCommit"] = canCommentOnCommit(ctx)
	ctx.Data["DiffNewCommentURL"] = commitCommentURL(ctx, comment.CommitSHA)
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")
	ctx.Data["comments"] = comments
	ctx.HTML(http.StatusOK, tplCommitConversation)
}

// DeleteCommitComment deletes an inline commit comment.
func DeleteCommitComment(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	commentID := ctx.PathParamInt64("id")
	if commentID <= 0 {
		ctx.NotFound(nil)
		return
	}

	comment, err := issues_model.GetCommitCommentByID(ctx, ctx.Repo.Repository.ID, commentID)
	if err != nil {
		ctx.NotFoundOrServerError("GetCommitCommentByID", db.IsErrNotExist, err)
		return
	}

	canDelete := comment.PosterID == ctx.Doer.ID ||
		ctx.Repo.Permission.IsAdmin() ||
		ctx.Repo.Permission.CanWrite(unit_model.TypeCode)
	if !canDelete {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	if err := issues_model.DeleteCommitComment(ctx, ctx.Repo.Repository.ID, commentID); err != nil {
		ctx.ServerError("DeleteCommitComment", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}
