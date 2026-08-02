// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	activities_model "gitea.dev/models/activities"
	"gitea.dev/models/db"
	"gitea.dev/models/renderhelper"
	repo_model "gitea.dev/models/repo"
	unit_model "gitea.dev/models/unit"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/references"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
	"gitea.dev/services/gitdiff"
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

// renderCommitCommentContents renders the markdown body of every comment
// attached to a single file.
func renderCommitCommentContents(ctx *context.Context, fcc *repo_model.FileCommitComments) {
	if fcc == nil {
		return
	}
	for _, side := range []map[int][]*repo_model.CommitComment{fcc.Left, fcc.Right} {
		for _, comments := range side {
			for _, c := range comments {
				rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository, renderhelper.RepoCommentOptions{})
				var err error
				if c.RenderedContent, err = markdown.RenderString(rctx, c.Content); err != nil {
					log.Error("RenderString for commit comment %d: %v", c.ID, err)
				}
			}
		}
	}
}

// renderCommitComments renders the markdown body of every comment in a diff.
func renderCommitComments(ctx *context.Context, all repo_model.CommitCommentsForDiff) {
	for _, fcc := range all {
		renderCommitCommentContents(ctx, fcc)
	}
}

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

	commit, err := ctx.Repo.GitRepo.GetCommit(ctx, commitSHA)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetCommit", err)
		}
		return
	}
	fullSHA := commit.ID.String()

	var parentSHA string
	if commit.ParentCount() > 0 {
		parentID, err := commit.ParentID(0)
		if err == nil {
			parentSHA = parentID.String()
		}
	}

	// Root commits (no parent) only have a "new" side, so reject any
	// comment that targets the old side of a non-existent diff.
	if parentSHA == "" && line < 0 {
		ctx.JSONError("cannot comment on the previous side of a root commit")
		return
	}

	// Generate diff context patch around the commented line. For commits
	// with a parent we diff against it; for root commits we still
	// validate against the file tree via the unchanged-line fallback so
	// that a crafted POST cannot create an invisible comment with no
	// real coordinate.
	var patch string
	if parentSHA != "" {
		absLine := line
		isOld := line < 0
		if isOld {
			absLine = -line
		}
		patch, err = git.GetFileDiffCutAroundLine(
			ctx, ctx.Repo.GitRepo, parentSHA, fullSHA, treePath,
			absLine, isOld, setting.UI.CodeCommentLines,
		)
		if err != nil {
			log.Debug("GetFileDiffCutAroundLine failed for commit comment: %v", err)
		}
	}
	if patch == "" {
		// Old-side coordinates only exist in the parent tree; resolving them
		// against fullSHA rejects lines the commit removed and stores the
		// wrong context for the ones it kept.
		contextSHA := fullSHA
		if line < 0 {
			contextSHA = parentSHA
		}
		patch, err = gitdiff.GeneratePatchForUnchangedLine(ctx, ctx.Repo.GitRepo, contextSHA, treePath, line, setting.UI.CodeCommentLines)
		if err != nil {
			log.Debug("GeneratePatchForUnchangedLine failed for commit comment: %v", err)
		}
	}

	if patch == "" {
		ctx.JSONError("comment coordinates do not resolve to a line in this commit")
		return
	}

	comment := &repo_model.CommitComment{
		RepoID:    ctx.Repo.Repository.ID,
		CommitSHA: fullSHA,
		TreePath:  treePath,
		Line:      line,
		PosterID:  ctx.Doer.ID,
		Poster:    ctx.Doer,
		Content:   content,
		Patch:     patch,
	}

	if err := repo_model.CreateCommitComment(ctx, comment); err != nil {
		ctx.ServerError("CreateCommitComment", err)
		return
	}

	// Send notifications to commit author and @mentioned users
	mentions := references.FindAllMentionsMarkdown(content)
	if err := activities_model.CreateCommitCommentNotification(ctx, ctx.Doer, ctx.Repo.Repository, fullSHA, comment.ID, commit.Author.Email, mentions); err != nil {
		log.Error("CreateCommitCommentNotification: %v", err)
	}

	// The response replaces the whole conversation holder client-side, so it
	// has to carry the entire thread and not just the comment just created.
	comments, err := repo_model.FindCommitCommentsByLine(ctx, ctx.Repo.Repository.ID, fullSHA, treePath, line)
	if err != nil {
		ctx.ServerError("FindCommitCommentsByLine", err)
		return
	}
	for _, c := range comments {
		rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository, renderhelper.RepoCommentOptions{})
		if c.RenderedContent, err = markdown.RenderString(rctx, c.Content); err != nil {
			log.Error("RenderString for commit comment %d: %v", c.ID, err)
		}
	}

	ctx.Data["CommitID"] = fullSHA
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
		if db.IsErrNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetCommitCommentByID", err)
		}
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
