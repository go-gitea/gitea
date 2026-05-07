// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	commit_service "code.gitea.io/gitea/services/commit"
)

// ListCommitComments returns all comments posted on a commit.
func ListCommitComments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/commits/{sha}/comments repository repoListCommitComments
	// ---
	// summary: List all comments posted on a commit
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: sha
	//   in: path
	//   description: commit SHA to list comments for
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/CommentList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !ctx.Repo.Permission.CanRead(unit.TypeCode) {
		ctx.APIErrorNotFound()
		return
	}

	sha := ctx.PathParam("sha")
	if sha == "" {
		ctx.APIError(http.StatusBadRequest, errors.New("missing commit SHA"))
		return
	}

	comments, err := commit_service.ListCommitComments(ctx, ctx.Repo.Repository, sha)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiComments := make([]*api.Comment, 0, len(comments))
	for _, c := range comments {
		if err := c.LoadPoster(ctx); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		if err := c.LoadAttachments(ctx); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		apiComments = append(apiComments, convert.ToAPIComment(ctx, ctx.Repo.Repository, c))
	}
	ctx.JSON(http.StatusOK, &apiComments)
}

// CreateCommitComment creates a new comment on a commit.
func CreateCommitComment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/commits/{sha}/comments repository repoCreateCommitComment
	// ---
	// summary: Add a comment to a commit
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: sha
	//   in: path
	//   description: commit SHA to comment on
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateCommitCommentOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Comment"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	if !ctx.Repo.Permission.CanRead(unit.TypeCode) {
		ctx.APIErrorNotFound()
		return
	}

	form := web.GetForm(ctx).(*api.CreateCommitCommentOption)
	sha := ctx.PathParam("sha")
	if sha == "" {
		ctx.APIError(http.StatusBadRequest, errors.New("missing commit SHA"))
		return
	}

	comment, err := commit_service.CreateCommitComment(ctx, commit_service.CreateCommitCommentOptions{
		Doer:        ctx.Doer,
		Repo:        ctx.Repo.Repository,
		CommitSHA:   sha,
		Content:     form.Body,
		TreePath:    form.Path,
		Line:        form.Line,
		Attachments: form.Attachments,
	})
	if err != nil {
		var notFound commit_service.ErrCommitNotFound
		switch {
		case errors.Is(err, user_model.ErrBlockedUser):
			ctx.APIError(http.StatusForbidden, err)
		case errors.As(err, &notFound):
			ctx.APIErrorNotFound(err)
		default:
			ctx.APIErrorInternal(err)
		}
		return
	}

	if err := comment.LoadPoster(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if err := comment.LoadAttachments(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToAPIComment(ctx, ctx.Repo.Repository, comment))
}
