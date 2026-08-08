// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"strconv"

	"gitea.dev/models/db"
	"gitea.dev/models/renderhelper"
	repo_model "gitea.dev/models/repo"
	unit_model "gitea.dev/models/unit"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
	repo_service "gitea.dev/services/repository"
)

var (
	tplNewCommitComment   templates.TplName = "repo/diff/new_commit_comment"
	tplCommitConversation templates.TplName = "repo/diff/commit_conversation"
)

// canCommentOnCommit mirrors the guards on the commit comment routes so the
// templates and the endpoints can never disagree about who may comment.
func canCommentOnCommit(ctx *context.Context) bool {
	return ctx.Doer != nil && !ctx.Repo.Repository.IsArchived && ctx.Repo.Permission.CanRead(unit_model.TypeCode)
}

// commitCommentURL is the endpoint the diff templates post new comments to.
func commitCommentURL(ctx *context.Context, commitSHA string) string {
	return ctx.Repo.RepoLink + "/commit/" + commitSHA + "/comment"
}

// renderCommitComments renders the markdown body of every given comment.
func renderCommitComments(ctx *context.Context, comments repo_model.CommitCommentList) {
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

// RenderNewCommitCommentForm renders the comment form for inline commit comments.
func RenderNewCommitCommentForm(ctx *context.Context) {
	commitSHA := ctx.PathParam("sha")
	ctx.Data["CanCommentOnCommit"] = canCommentOnCommit(ctx)
	ctx.Data["DiffNewCommentURL"] = commitCommentURL(ctx, commitSHA)
	ctx.HTML(http.StatusOK, tplNewCommitComment)
}

// CreateCommitComment handles creating an inline comment on a commit diff.
func CreateCommitComment(ctx *context.Context) {
	commitSHA := ctx.PathParam("sha")
	if commitSHA == "" {
		ctx.NotFound(nil)
		return
	}

	content := ctx.FormString("content")
	treePath := ctx.FormString("path")
	side := ctx.FormString("side")
	line := ctx.FormInt64("line")

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

	comment, err := repo_service.CreateCommitComment(ctx, ctx.Doer, ctx.Repo.Repository, ctx.Repo.GitRepo, commitSHA, treePath, line, content)
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

	// The response replaces the whole conversation holder client-side, so it
	// has to carry the entire thread and not just the comment just created.
	comments, err := repo_model.FindCommitCommentsByLine(ctx, ctx.Repo.Repository.ID, comment.CommitSHA, treePath, line)
	if err != nil {
		ctx.ServerError("FindCommitCommentsByLine", err)
		return
	}
	renderCommitComments(ctx, comments)

	ctx.Data["CanCommentOnCommit"] = canCommentOnCommit(ctx)
	ctx.Data["DiffNewCommentURL"] = commitCommentURL(ctx, comment.CommitSHA)
	ctx.Data["comments"] = comments
	ctx.HTML(http.StatusOK, tplCommitConversation)
}

// DeleteCommitComment handles deleting an inline comment on a commit.
func DeleteCommitComment(ctx *context.Context) {
	commentID := ctx.PathParamInt64("id")
	if commentID <= 0 {
		ctx.NotFound(nil)
		return
	}

	comment, err := repo_model.GetCommitCommentByID(ctx, ctx.Repo.Repository.ID, commentID)
	if err != nil {
		ctx.NotFoundOrServerError("GetCommitCommentByID", db.IsErrNotExist, err)
		return
	}

	if comment.PosterID != ctx.Doer.ID && !ctx.Repo.Permission.IsAdmin() {
		ctx.JSONError("permission denied")
		return
	}

	if err := repo_model.DeleteCommitComment(ctx, ctx.Repo.Repository.ID, commentID); err != nil {
		ctx.ServerError("DeleteCommitComment", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}
