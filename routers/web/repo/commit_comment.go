// SPDX-License-Identifier: MIT
package repo

import (
	"fmt"
	"net/http"
	"path"

	git_model "code.gitea.io/gitea/models/git"
	renderhelper "code.gitea.io/gitea/models/renderhelper"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
)

// DeleteCommitComment deletes a commit comment
func DeleteCommitComment(ctx *context.Context) {
	id := ctx.PathParamInt64("id")
	cc, err := git_model.GetCommitCommentByID(ctx, id)
	if err != nil {
		ctx.NotFoundOrServerError("GetCommitCommentByID", git_model.IsErrCommitCommentNotExist, err)
		return
	}

	if cc.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(err)
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != cc.PosterID && !ctx.Repo.CanWriteToBranch(ctx, ctx.Doer, ctx.Repo.BranchName)) {
		// allow deletion by poster or users who can write to the branch (repo maintainers)
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	if err := git_model.DeleteCommitComment(ctx, id); err != nil {
		ctx.ServerError("DeleteCommitComment", err)
		return
	}

	log.Trace("Commit comment deleted: %d/%d", ctx.Repo.Repository.ID, id)
	ctx.Status(http.StatusOK)
}

// UpdateCommitComment updates commit comment content
func UpdateCommitComment(ctx *context.Context) {
	id := ctx.PathParamInt64("id")
	cc, err := git_model.GetCommitCommentByID(ctx, id)
	if err != nil {
		ctx.NotFoundOrServerError("GetCommitCommentByID", git_model.IsErrCommitCommentNotExist, err)
		return
	}

	if cc.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(err)
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != cc.PosterID && !ctx.Repo.CanWriteToBranch(ctx, ctx.Doer, ctx.Repo.BranchName)) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	newContent := ctx.FormString("content")
	if newContent != cc.Content {
		oldContent := cc.Content
		cc.Content = newContent
		if err := git_model.UpdateCommitComment(ctx, cc); err != nil {
			ctx.ServerError("UpdateCommitComment", err)
			return
		}
		log.Trace("Commit comment updated: %d/%d", ctx.Repo.Repository.ID, id)
		_ = oldContent // reserved for potential audit/logging
	}

	// render updated content using markdown renderer so newlines and markdown are preserved
	rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository, renderhelper.RepoCommentOptions{CurrentRefPath: path.Join("commit", util.PathEscapeSegments(cc.CommitSHA))})
	renderedHTML, err := markdown.RenderString(rctx, cc.Content)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"content":        renderedHTML,
		"contentVersion": cc.ContentVersion(),
		"attachments":    "",
	})
}

// ChangeCommitCommentReaction handles react/unreact on a commit comment
func ChangeCommitCommentReaction(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.ReactionForm)
	id := ctx.PathParamInt64("id")
	cc, err := git_model.GetCommitCommentByID(ctx, id)
	if err != nil {
		ctx.NotFoundOrServerError("GetCommitCommentByID", git_model.IsErrCommitCommentNotExist, err)
		return
	}

	if cc.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(err)
		return
	}

	if !ctx.IsSigned {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	switch ctx.PathParam("action") {
	case "react":
		if _, err := git_model.CreateCommitCommentReaction(ctx, ctx.Doer, cc.ID, form.Content); err != nil {
			// log and continue; forbidden reaction returns error
			log.Info("CreateCommitCommentReaction: %s", err)
			break
		}
	case "unreact":
		if err := git_model.DeleteCommitCommentReaction(ctx, ctx.Doer.ID, cc.ID, form.Content); err != nil {
			ctx.ServerError("DeleteCommitCommentReaction", err)
			return
		}
	default:
		ctx.NotFound(nil)
		return
	}

	// Reload new reactions
	reactions, err := git_model.LoadReactionsForCommitComment(ctx, cc.ID)
	if err != nil {
		ctx.ServerError("LoadReactionsForCommitComment", err)
		return
	}

	// Log reactions counts for diagnostics
	var totalReacts int
	for _, list := range reactions {
		totalReacts += len(list)
	}
	log.Trace("Loaded %d commit comment reactions for: %d", totalReacts, cc.ID)

	html, err := ctx.RenderToHTML(tplReactions, map[string]any{
		"ActionURL": fmt.Sprintf("%s/commit/%s/comments/%d/reactions", ctx.Repo.RepoLink, cc.CommitSHA, cc.ID),
		"Reactions": reactions,
	})
	if err != nil {
		ctx.ServerError("ChangeCommitCommentReaction.HTMLString", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{
		"html": html,
	})
}
