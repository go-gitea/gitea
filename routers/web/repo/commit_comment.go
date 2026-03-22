// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strconv"

	"code.gitea.io/gitea/models/renderhelper"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/context/upload"
	"code.gitea.io/gitea/services/forms"
	repo_service "code.gitea.io/gitea/services/repository"
)

const (
	tplCommitNewComment       templates.TplName = "repo/diff/commit_new_comment"
	tplCommitDiffConversation templates.TplName = "repo/diff/commit_conversation"
)

// RenderNewCommitCommentForm renders the form for creating a new commit inline comment
func RenderNewCommitCommentForm(ctx *context.Context) {
	ctx.Data["PageIsCommitDiff"] = true
	ctx.Data["CommitSHA"] = ctx.PathParam("sha")
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")
	ctx.HTML(http.StatusOK, tplCommitNewComment)
}

// CreateCommitComment creates an inline comment on a commit diff
func CreateCommitComment(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CommitCodeCommentForm)
	commitSHA := ctx.PathParam("sha")

	if ctx.HasError() {
		ctx.Flash.Error(ctx.Data["ErrorMsg"].(string))
		ctx.Redirect(ctx.Repo.RepoLink + "/commit/" + commitSHA)
		return
	}

	signedLine := form.Line
	if form.Side == "previous" {
		signedLine *= -1
	}

	comment, err := repo_service.CreateCommitCodeComment(ctx,
		ctx.Doer,
		ctx.Repo.Repository,
		ctx.Repo.GitRepo,
		commitSHA,
		form.TreePath,
		form.Content,
		signedLine,
	)
	if err != nil {
		ctx.ServerError("CreateCommitCodeComment", err)
		return
	}

	renderCommitConversation(ctx, comment, commitSHA)
}

func renderCommitConversation(ctx *context.Context, comment *repo_model.CommitCodeComment, commitSHA string) {
	comments, err := repo_model.FetchCommitCodeCommentsByLine(ctx, ctx.Repo.Repository.ID, commitSHA, comment.TreePath, comment.Line)
	if err != nil {
		ctx.ServerError("FetchCommitCodeCommentsByLine", err)
		return
	}

	for _, c := range comments {
		if err := c.LoadPoster(ctx); err != nil {
			ctx.ServerError("LoadPoster", err)
			return
		}
		rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository, renderhelper.RepoCommentOptions{
			FootnoteContextID: strconv.FormatInt(c.ID, 10),
		})
		if c.RenderedContent, err = markdown.RenderString(rctx, c.Content); err != nil {
			ctx.ServerError("RenderString", err)
			return
		}
	}

	ctx.Data["PageIsCommitDiff"] = true
	ctx.Data["CommitSHA"] = commitSHA
	ctx.Data["comments"] = comments
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	ctx.Data["CanBlockUser"] = func(blocker, blockee *user_model.User) bool {
		return false
	}
	upload.AddUploadContext(ctx, "comment")
	ctx.HTML(http.StatusOK, tplCommitDiffConversation)
}
