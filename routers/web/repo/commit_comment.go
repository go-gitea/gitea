// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/web"
	commit_service "code.gitea.io/gitea/services/commit"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
)

// NewCommitComment posts a new comment on a commit from the commit detail page.
// Mirrors the web-side issue-comment flow, but anchors to a SHA rather than to
// an Issue.Index. The synthetic carrier Issue is created lazily by the service
// the first time anyone comments on the SHA.
func NewCommitComment(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateCommitCommentForm)
	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	sha := ctx.PathParam("sha")
	if sha == "" {
		ctx.HTTPError(http.StatusBadRequest)
		return
	}

	if !ctx.IsSigned {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	comment, err := commit_service.CreateCommitComment(ctx, commit_service.CreateCommitCommentOptions{
		Doer:        ctx.Doer,
		Repo:        ctx.Repo.Repository,
		CommitSHA:   sha,
		Content:     form.Content,
		TreePath:    form.Path,
		Line:        form.Line,
		Attachments: form.Files,
	})
	if err != nil {
		var notFound commit_service.ErrCommitNotFound
		switch {
		case errors.Is(err, user_model.ErrBlockedUser):
			ctx.HTTPError(http.StatusForbidden)
		case errors.As(err, &notFound):
			ctx.HTTPError(http.StatusNotFound)
		default:
			ctx.ServerError("CreateCommitComment", err)
		}
		return
	}

	// JSON response keeps the form-submission flow consistent with the issue
	// comment form (which also uses ctx.JSON / ctx.JSONError so the page can
	// re-render only the comment timeline rather than full-reloading).
	ctx.JSON(http.StatusOK, map[string]any{
		"comment_id": comment.ID,
	})
}
