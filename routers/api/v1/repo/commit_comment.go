

// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/commit"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	notify_service "code.gitea.io/gitea/services/notify"
)

// ListCommitComments list comments on a commit
func ListCommitComments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/commits/{sha}/comments repository repoListCommitComments
	// ---
	// summary: List comments on a commit
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
	//   description: SHA of the commit
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/CommentList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	sha := ctx.PathParam("sha")
	if len(sha) == 0 {
		ctx.Error(http.StatusBadRequest, "SHA is empty")
		return
	}

	// Get the commit
	commit, err := ctx.Repo.GitRepo.GetCommit(sha)
	if err != nil {
		ctx.Error(http.StatusNotFound, "GetCommit", err)
		return
	}

	// Check if user has permission to read this repo
	if !ctx.Repo.Permission.CanRead(repo_model.UnitTypeCode) {
		ctx.Error(http.StatusForbidden, "No permission to read code", nil)
		return
	}

	// Get comments for this commit
	comments, err := repo_model.GetCommitComments(ctx, ctx.Repo.Repository.ID, sha)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetCommitComments", err)
		return
	}

	// Load posters
	for _, comment := range comments {
		if err := comment.LoadPoster(ctx); err != nil {
			ctx.Error(http.StatusInternalServerError, "LoadPoster", err)
			return
		}
	}

	// Convert to API format
	apiComments := make([]*api.Comment, len(comments))
	for i, comment := range comments {
		apiComments[i] = convert.ToAPIComment(ctx, ctx.Repo.Repository, &repo_model.Comment{
			ID:          comment.ID,
			PosterID:    comment.PosterID,
			Poster:      comment.Poster,
			Content:     comment.Content,
			CreatedUnix: comment.CreatedUnix,
			UpdatedUnix: comment.UpdatedUnix,
			Type:        repo_model.CommentTypeCommit,
		})
	}

	ctx.JSON(http.StatusOK, &apiComments)
}

// CreateCommitComment create a comment on a commit
func CreateCommitComment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/commits/{sha}/comments repository repoCreateCommitComment
	// ---
	// summary: Create a comment on a commit
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
	//   description: SHA of the commit
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

	form := web.GetForm(ctx).(*api.CreateCommitCommentOption)
	sha := ctx.PathParam("sha")

	if len(sha) == 0 {
		ctx.Error(http.StatusBadRequest, "SHA is empty", nil)
		return
	}

	// Get the commit
	commit, err := ctx.Repo.GitRepo.GetCommit(sha)
	if err != nil {
		ctx.Error(http.StatusNotFound, "GetCommit", err)
		return
	}

	// Check if user has permission to comment on this repo
	if !ctx.Repo.Permission.CanRead(repo_model.UnitTypeCode) {
		ctx.Error(http.StatusForbidden, "No permission to read code", nil)
		return
	}

	// Create the comment
	comment, err := commit.CreateCommitComment(ctx, ctx.Doer, ctx.Repo.Repository, sha, form.Line, form.Path, form.Body, form.Attachments)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateCommitComment", err)
		return
	}

	// Load poster
	if err := comment.LoadPoster(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadPoster", err)
		return
	}

	// Notify subscribers
	notify_service.CreateCommitComment(ctx, ctx.Doer, ctx.Repo.Repository, sha, comment)

	// Convert to API format
	apiComment := convert.ToAPIComment(ctx, ctx.Repo.Repository, &repo_model.Comment{
		ID:          comment.ID,
		PosterID:    comment.PosterID,
		Poster:      comment.Poster,
		Content:     comment.Content,
		CreatedUnix: comment.CreatedUnix,
		UpdatedUnix: comment.UpdatedUnix,
		Type:        repo_model.CommentTypeCommit,
	})

	ctx.JSON(http.StatusCreated, apiComment)
}

// DeleteCommitComment delete a commit comment
func DeleteCommitComment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/commits/comments/{id} repository repoDeleteCommitComment
	// ---
	// summary: Delete a commit comment
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
	// - name: id
	//   in: path
	//   description: id of comment to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// Get the comment
	comment, err := repo_model.GetCommitCommentByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.Error(http.StatusNotFound, "GetCommitCommentByID", err)
		return
	}

	// Check if user has permission to delete this comment
	if !ctx.Repo.Permission.CanWrite(repo_model.UnitTypeCode) && ctx.Doer.ID != comment.PosterID {
		ctx.Error(http.StatusForbidden, "No permission to delete comment", nil)
		return
	}

	// Notify subscribers
	notify_service.DeleteCommitComment(ctx, ctx.Doer, comment)

	// Delete the comment
	if err := commit.DeleteCommitComment(ctx, ctx.Doer, comment); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteCommitComment", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

