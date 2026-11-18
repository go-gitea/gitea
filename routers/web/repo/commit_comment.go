// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"html/template"
	"net/http"
	"strconv"

	git_model "gitea.dev/models/git"
	"gitea.dev/models/renderhelper"
	"gitea.dev/modules/git"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/context/upload"
	"gitea.dev/services/convert"
	"gitea.dev/services/forms"
)

const (
	tplCommitConversation templates.TplName = "repo/diff/commit_conversation"
	tplNewCommitComment   templates.TplName = "repo/diff/new_commit_comment"
)

// SetCommitCommentContext prepares the page data for adding comments to a commit diff
func SetCommitCommentContext(ctx *context.Context, commitSHA string) {
	ctx.Data["CommitCommentSHA"] = commitSHA
	ctx.Data["CanCommentOnCommit"] = ctx.IsSigned && !ctx.Repo.Repository.IsArchived
	upload.AddUploadContext(ctx, "commit_comment")
}

func loadCommitComments(ctx *context.Context, comments []*git_model.CommitComment) error {
	if err := git_model.LoadCommitCommentsAttachments(ctx, comments); err != nil {
		return err
	}
	for _, c := range comments {
		if err := c.LoadPoster(ctx); err != nil {
			return err
		}
		html, err := renderCommitCommentContent(ctx, c)
		if err != nil {
			return err
		}
		c.RenderedContent = html
	}
	return nil
}

func renderCommitCommentContent(ctx *context.Context, c *git_model.CommitComment) (template.HTML, error) {
	if c.Content == "" {
		return "", nil
	}
	rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository, renderhelper.RepoCommentOptions{
		FootnoteContextID: "commit-" + strconv.FormatInt(c.ID, 10),
	})
	return markdown.RenderString(rctx, c.Content)
}

// LoadCommitCommentsForDiff loads the comments of the commit for rendering in the diff
func LoadCommitCommentsForDiff(ctx *context.Context, commitSHA string) ([]*git_model.CommitComment, error) {
	comments, err := git_model.ListCommitComments(ctx, ctx.Repo.Repository.ID, commitSHA)
	if err != nil {
		return nil, err
	}
	return comments, loadCommitComments(ctx, comments)
}

func getCommitCommentForChange(ctx *context.Context) *git_model.CommitComment {
	c, err := git_model.GetCommitCommentByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if errors.Is(err, git_model.ErrCommitCommentNotExist) {
			ctx.NotFound()
		} else {
			ctx.ServerErrorAuto(err)
		}
		return nil
	}
	if c.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound()
		return nil
	}
	return c
}

func getCommitCommentForSelfOrAdmin(ctx *context.Context) *git_model.CommitComment {
	c := getCommitCommentForChange(ctx)
	if c == nil {
		return nil
	}
	if c.PosterID != ctx.Doer.ID && !ctx.Repo.Permission.IsAdmin() {
		ctx.HTTPError(http.StatusForbidden)
		return nil
	}
	return c
}

// NewCommitCommentForm renders the form for a new comment on a commit diff line
func NewCommitCommentForm(ctx *context.Context) {
	SetCommitCommentContext(ctx, ctx.PathParam("sha"))
	ctx.HTML(http.StatusOK, tplNewCommitComment)
}

func renderCommitConversation(ctx *context.Context, commitSHA string, c *git_model.CommitComment) {
	all, err := git_model.ListCommitComments(ctx, c.RepoID, commitSHA)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	var comments []*git_model.CommitComment
	for _, other := range all {
		if other.TreePath == c.TreePath && other.Line == c.Line {
			comments = append(comments, other)
		}
	}
	if err := loadCommitComments(ctx, comments); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	SetCommitCommentContext(ctx, commitSHA)
	ctx.Data["comments"] = comments
	ctx.HTML(http.StatusOK, tplCommitConversation)
}

// CreateCommitComment creates a code comment on a commit
func CreateCommitComment(ctx *context.Context) {
	form := web.GetForm[*forms.CommitCommentForm](ctx)
	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	commit, err := ctx.Repo.GitRepo.GetCommit(ctx, ctx.PathParam("sha"))
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.ServerErrorAuto(err)
		}
		return
	}

	line := form.Line
	if form.Side == "previous" {
		line = -line
	}
	c := &git_model.CommitComment{
		RepoID:    ctx.Repo.Repository.ID,
		CommitSHA: commit.ID.String(),
		PosterID:  ctx.Doer.ID,
		Poster:    ctx.Doer,
		TreePath:  form.TreePath,
		Line:      line,
		Content:   form.Content,
	}
	var files []string
	if setting.Attachment.Enabled {
		files = form.Files
	}
	if err := git_model.CreateCommitComment(ctx, c, files); err != nil {
		ctx.JSONErrorAuto(err)
		return
	}
	renderCommitConversation(ctx, c.CommitSHA, c)
}

// UpdateCommitComment changes the content of a commit comment
func UpdateCommitComment(ctx *context.Context) {
	c := getCommitCommentForSelfOrAdmin(ctx)
	if c == nil {
		return
	}

	c.Content = ctx.FormString("content")
	if err := git_model.UpdateCommitComment(ctx, c, ctx.FormInt("content_version"), ctx.FormStrings("files[]"), setting.Attachment.Enabled); err != nil {
		if errors.Is(err, git_model.ErrCommitCommentAlreadyChanged) {
			ctx.JSONError(ctx.Tr("repo.comments.edit.already_changed"))
		} else {
			ctx.ServerErrorAuto(err)
		}
		return
	}

	rendered, err := renderCommitCommentContent(ctx, c)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{
		"content":        commentContentHTML(ctx, rendered),
		"contentVersion": c.ContentVersion,
		"attachments":    attachmentsHTML(ctx, c.Attachments, c.Content),
	})
}

// DeleteCommitComment deletes a commit comment
func DeleteCommitComment(ctx *context.Context) {
	c := getCommitCommentForSelfOrAdmin(ctx)
	if c == nil {
		return
	}
	if err := git_model.DeleteCommitComment(ctx, c); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.Status(http.StatusOK)
}

// GetCommitCommentAttachments lists the attachments of a commit comment
func GetCommitCommentAttachments(ctx *context.Context) {
	c := getCommitCommentForChange(ctx)
	if c == nil {
		return
	}
	if err := c.LoadAttachments(ctx); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	attachments := make([]*api.Attachment, 0, len(c.Attachments))
	for _, a := range c.Attachments {
		attachments = append(attachments, convert.ToAttachment(ctx, ctx.Repo.Repository, a))
	}
	ctx.JSON(http.StatusOK, attachments)
}
