// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"gitea.dev/models/renderhelper"
	repo_model "gitea.dev/models/repo"
	unit_model "gitea.dev/models/unit"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
	"gitea.dev/services/context/upload"
	"gitea.dev/services/gitdiff"
)

const (
	tplNewCommitComment   templates.TplName = "repo/diff/new_commit_comment"
	tplCommitConversation templates.TplName = "repo/diff/commit_conversation"
)

// RenderNewCommitCommentForm renders the form for creating a new comment on a commit
func RenderNewCommitCommentForm(ctx *context.Context) {
	ctx.Data["PageIsPullFiles"] = true
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")
	ctx.HTML(http.StatusOK, tplNewCommitComment)
}

// CreateCommitComment creates a new inline comment on a commit
func CreateCommitComment(ctx *context.Context) {
	commitID := ctx.PathParam("sha")

	side := ctx.FormString("side")
	line := ctx.FormInt64("line")
	treePath := ctx.FormString("path")
	content := ctx.FormString("content")

	signedLine := line
	if side == "previous" {
		signedLine *= -1
	}

	if signedLine == 0 {
		ctx.JSONError("line must be non-zero")
		return
	}

	if content == "" {
		ctx.JSONError("content is required")
		return
	}

	if treePath == "" {
		ctx.JSONError("path is required")
		return
	}

	var patch string
	gitRepo := ctx.Repo.GitRepo

	if gitRepo != nil {
		patch, _ = git.GetFileDiffCutAroundLine(
			gitRepo, commitID, commitID, treePath,
			int64((&repo_model.CommitComment{Line: signedLine}).UnsignedLine()), signedLine < 0, setting.UI.CodeCommentLines,
		)

		if patch == "" {
			p, err := gitdiff.GeneratePatchForUnchangedLine(gitRepo, commitID, treePath, signedLine, setting.UI.CodeCommentLines)
			if err == nil {
				patch = p
			} else {
				log.Debug("Unable to generate patch for commit comment (file=%s, line=%d, commit=%s): %v", treePath, signedLine, commitID, err)
			}
		}
	}

	comment, err := repo_model.CreateCommitComment(ctx, ctx.Doer.ID, ctx.Repo.Repository.ID, commitID, treePath, signedLine, content, patch)
	if err != nil {
		ctx.ServerError("CreateCommitComment", err)
		return
	}

	log.Trace("Commit comment created: %-v Comment[%d]", ctx.Repo.Repository, comment.ID)

	renderCommitConversation(ctx, comment)
}

// DeleteCommitComment deletes a commit comment
func DeleteCommitComment(ctx *context.Context) {
	commentIDStr := ctx.PathParam("id")
	commentID, err := strconv.ParseInt(commentIDStr, 10, 64)
	if err != nil {
		ctx.ServerError("Invalid comment ID", err)
		return
	}

	comment, err := repo_model.FindCommitCommentByID(ctx, commentID)
	if err != nil {
		ctx.NotFound(err)
		return
	}

	if comment.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(fmt.Errorf("commit comment repoID mismatch"))
		return
	}

	if comment.PosterID != ctx.Doer.ID && !ctx.Repo.Permission.CanWrite(unit_model.TypeCode) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	if err := repo_model.DeleteCommitComment(ctx, commentID, ctx.Repo.Repository.ID); err != nil {
		ctx.ServerError("DeleteCommitComment", err)
		return
	}

	ctx.JSONOK()
}

func renderCommitConversation(ctx *context.Context, comment *repo_model.CommitComment) {
	comments, err := repo_model.FindCommitCommentsByCommitSHA(ctx, ctx.Repo.Repository.ID, comment.CommitSHA)
	if err != nil {
		ctx.ServerError("FindCommitCommentsByCommitSHA", err)
		return
	}

	if err := comments.LoadPosters(ctx); err != nil {
		ctx.ServerError("LoadPosters", err)
		return
	}

	rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository, renderhelper.RepoCommentOptions{
		CurrentRefSubURL: "commit/" + comment.CommitSHA,
	})

	for _, c := range comments {
		rendered, err := markdown.RenderString(rctx, c.Content)
		if err != nil {
			log.Error("RenderString: %v", err)
			c.RenderedContent = template.HTML(template.HTMLEscapeString(c.Content))
		} else {
			c.RenderedContent = rendered
		}
	}

	var lineComments []*repo_model.CommitComment
	for _, c := range comments {
		if c.TreePath == comment.TreePath && c.Line == comment.Line {
			lineComments = append(lineComments, c)
		}
	}

	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")
	ctx.Data["comments"] = lineComments
	ctx.HTML(http.StatusOK, tplCommitConversation)
}
