// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	activities_model "code.gitea.io/gitea/models/activities"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/renderhelper"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/references"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/gitdiff"
)

var (
	tplNewCommitComment   templates.TplName = "repo/diff/new_commit_comment"
	tplCommitConversation templates.TplName = "repo/diff/commit_conversation"
)

// RenderNewCommitCommentForm renders the comment form for inline commit comments.
func RenderNewCommitCommentForm(ctx *context.Context) {
	commitSHA := ctx.PathParam("sha")
	ctx.Data["CommitID"] = commitSHA
	ctx.Data["PageIsDiff"] = true
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
	treePath := ctx.FormString("tree_path")
	side := ctx.FormString("side")
	line := ctx.FormInt64("line")

	if content == "" || treePath == "" || line == 0 {
		ctx.JSONError("content, tree_path, and line are required")
		return
	}

	if side == "previous" {
		line = -line
	}

	commit, err := ctx.Repo.GitRepo.GetCommit(commitSHA)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetCommit", err)
		}
		return
	}
	fullSHA := commit.ID.String()

	// Generate diff context patch around the commented line
	var patch string
	var parentSHA string
	if commit.ParentCount() > 0 {
		parentID, err := commit.ParentID(0)
		if err == nil {
			parentSHA = parentID.String()
		}
	}
	if parentSHA != "" {
		absLine := line
		isOld := line < 0
		if isOld {
			absLine = -line
		}
		patch, err = git.GetFileDiffCutAroundLine(
			ctx.Repo.GitRepo, parentSHA, fullSHA, treePath,
			absLine, isOld, setting.UI.CodeCommentLines,
		)
		if err != nil {
			log.Debug("GetFileDiffCutAroundLine failed for commit comment: %v", err)
		}
		if patch == "" {
			patch, err = gitdiff.GeneratePatchForUnchangedLine(ctx.Repo.GitRepo, fullSHA, treePath, line, setting.UI.CodeCommentLines)
			if err != nil {
				log.Debug("GeneratePatchForUnchangedLine failed for commit comment: %v", err)
			}
		}
	}

	comment := &issues_model.Comment{
		Type:      issues_model.CommentTypeCommitComment,
		PosterID:  ctx.Doer.ID,
		Poster:    ctx.Doer,
		CommitSHA: fullSHA,
		TreePath:  treePath,
		Line:      line,
		Content:   content,
		Patch:     patch,
	}

	if err := issues_model.CreateCommitComment(ctx, ctx.Repo.Repository.ID, fullSHA, comment); err != nil {
		ctx.ServerError("CreateCommitComment", err)
		return
	}

	// Send notifications to commit author and @mentioned users
	mentions := references.FindAllMentionsMarkdown(content)
	if err := activities_model.CreateCommitCommentNotification(ctx, ctx.Doer, ctx.Repo.Repository, comment, commit.Author.Email, mentions); err != nil {
		log.Error("CreateCommitCommentNotification: %v", err)
	}

	// Render markdown content
	rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository, renderhelper.RepoCommentOptions{})
	comment.RenderedContent, err = markdown.RenderString(rctx, comment.Content)
	if err != nil {
		log.Error("RenderString for commit comment %d: %v", comment.ID, err)
	}

	ctx.Data["CommitID"] = fullSHA
	ctx.Data["comments"] = []*issues_model.Comment{comment}
	ctx.HTML(http.StatusOK, tplCommitConversation)
}

// DeleteCommitComment handles deleting an inline comment on a commit.
func DeleteCommitComment(ctx *context.Context) {
	commentID := ctx.PathParamInt64("id")
	if commentID <= 0 {
		ctx.NotFound(nil)
		return
	}

	comment, err := issues_model.GetCommitCommentByID(ctx, ctx.Repo.Repository.ID, commentID)
	if err != nil {
		ctx.NotFound(err)
		return
	}

	if comment.PosterID != ctx.Doer.ID && !ctx.Repo.IsAdmin() {
		ctx.JSONError("permission denied")
		return
	}

	if err := issues_model.DeleteCommitComment(ctx, commentID); err != nil {
		ctx.ServerError("DeleteCommitComment", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}
