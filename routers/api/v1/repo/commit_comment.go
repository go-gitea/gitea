// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	issue_service "gitea.dev/services/issue"
)

func ListRepoCommitComments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/comments repository listRepoCommitComments
	// ---
	// summary: List commit comments in a repository
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
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/CommitCommentList"
	//   default:
	//     "$ref": "#/responses/error"

	listCommitComments(ctx, "")
}

func ListCommitComments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/commits/{sha}/comments repository listCommitComments
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
	//   description: commit SHA
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/CommitCommentList"
	//   default:
	//     "$ref": "#/responses/error"

	if commit := getCommentCommit(ctx); commit != nil {
		listCommitComments(ctx, commit.ID.String())
	}
}

func listCommitComments(ctx *context.APIContext, commitSHA string) {
	listOptions := utils.GetListOptions(ctx)
	comments, count, err := issues_model.FindCommitComments(ctx, ctx.Repo.Repository.ID, commitSHA, listOptions)
	if err == nil {
		err = comments.LoadAttachments(ctx)
	}
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	result := make([]*api.CommitComment, len(comments))
	for index, comment := range comments {
		result[index] = convert.ToCommitComment(ctx, ctx.Repo.Repository, comment)
	}
	ctx.SetLinkHeader(count, listOptions.PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, result)
}

func getCommentCommit(ctx *context.APIContext) *git.Commit {
	sha := ctx.PathParam("sha")
	if !git.IsValidRefPattern(sha) {
		ctx.APIError(http.StatusUnprocessableEntity, "invalid commit SHA")
		return nil
	}
	commit, err := ctx.Repo.GitRepo.GetCommit(ctx, sha)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return nil
	}
	return commit
}

func CreateCommitComment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/commits/{sha}/comments repository createCommitComment
	// ---
	// summary: Create a commit comment using a file line number rather than a diff position
	// produces:
	// - application/json
	// consumes:
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
	//   description: commit SHA
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateCommitCommentOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/CommitComment"
	//   default:
	//     "$ref": "#/responses/error"

	if setting.Repository.DisableCommitComments {
		ctx.APIError(http.StatusForbidden, "commit comments are disabled")
		return
	}
	form := web.GetForm[*api.CreateCommitCommentOption](ctx)
	if form.Path != "" && form.Line <= 0 || form.Path == "" && (form.Line != 0 || form.Side != "") {
		ctx.APIError(http.StatusUnprocessableEntity, "path and a positive line must be provided together")
		return
	}
	commit := getCommentCommit(ctx)
	if commit == nil {
		return
	}
	line := form.Line
	if form.Side == "old" {
		line = -line
	}
	comment, err := issue_service.CreateCommitComment(ctx, ctx.Doer, ctx.Repo.Repository, commit, form.Path, line, form.Body, nil)
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToCommitComment(ctx, ctx.Repo.Repository, comment))
}

func getCommitComment(ctx *context.APIContext) *issues_model.Comment {
	comment, err := issues_model.GetRepoCommentByID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return nil
	}
	if !comment.IsCommitComment() {
		ctx.APIErrorNotFound()
		return nil
	}
	if err := comment.LoadPoster(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return nil
	}
	if err := comment.LoadAttachments(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return nil
	}
	return comment
}

func GetCommitComment(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/comments/{id} repository getCommitComment
	// ---
	// summary: Get a commit comment
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
	//   description: comment ID
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/CommitComment"
	//   default:
	//     "$ref": "#/responses/error"

	if comment := getCommitComment(ctx); comment != nil {
		ctx.JSON(http.StatusOK, convert.ToCommitComment(ctx, ctx.Repo.Repository, comment))
	}
}

func EditCommitComment(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/comments/{id} repository editCommitComment
	// ---
	// summary: Edit a commit comment
	// produces:
	// - application/json
	// consumes:
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
	//   description: comment ID
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditCommitCommentOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/CommitComment"
	//   default:
	//     "$ref": "#/responses/error"

	comment := getCommitComment(ctx)
	if comment == nil {
		return
	}
	if !issue_service.CanSelfChangeOrAccessComment(ctx.Doer, ctx.Repo.Permission, comment, perm.AccessModeWrite) {
		ctx.APIError(http.StatusForbidden, "no permission to edit comment")
		return
	}
	form := web.GetForm[*api.EditCommitCommentOption](ctx)
	if form.Body != nil && *form.Body != comment.Content {
		if *form.Body == "" {
			ctx.APIError(http.StatusUnprocessableEntity, "body must not be empty")
			return
		}
		oldContent := comment.Content
		comment.Content = *form.Body
		if err := issue_service.UpdateComment(ctx, ctx.Doer, ctx.Repo.Repository, comment, comment.ContentVersion, oldContent); err != nil {
			ctx.APIErrorAuto(err)
			return
		}
	}
	ctx.JSON(http.StatusOK, convert.ToCommitComment(ctx, ctx.Repo.Repository, comment))
}

func DeleteCommitComment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/comments/{id} repository deleteCommitComment
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
	//   description: comment ID
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   default:
	//     "$ref": "#/responses/error"

	comment := getCommitComment(ctx)
	if comment == nil {
		return
	}
	if !issue_service.CanSelfChangeOrAccessComment(ctx.Doer, ctx.Repo.Permission, comment, perm.AccessModeWrite) {
		ctx.APIError(http.StatusForbidden, "no permission to delete comment")
		return
	}
	if err := issue_service.DeleteComment(ctx, ctx.Doer, ctx.Repo.Repository, comment); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func GetCommitCommentReactions(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/comments/{id}/reactions repository getCommitCommentReactions
	// ---
	// summary: List reactions on a commit comment
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
	//   description: comment ID
	//   type: integer
	//   format: int64
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/ReactionList"
	//   default:
	//     "$ref": "#/responses/error"

	if comment := getCommitComment(ctx); comment != nil {
		listCommentReactions(ctx, issues_model.FindReactionsOptions{CommentID: comment.ID, ListOptions: utils.GetListOptions(ctx)})
	}
}

func PostCommitCommentReaction(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/comments/{id}/reactions repository postCommitCommentReaction
	// ---
	// summary: Add a reaction to a commit comment
	// produces:
	// - application/json
	// consumes:
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
	//   description: comment ID
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditReactionOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Reaction"
	//   "201":
	//     "$ref": "#/responses/Reaction"
	//   default:
	//     "$ref": "#/responses/error"

	if comment := getCommitComment(ctx); comment != nil {
		changeCommentReaction(ctx, comment, *web.GetForm[*api.EditReactionOption](ctx), true)
	}
}

func DeleteCommitCommentReaction(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/comments/{id}/reactions repository deleteCommitCommentReaction
	// ---
	// summary: Remove your reaction from a commit comment
	// produces:
	// - application/json
	// consumes:
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
	//   description: comment ID
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditReactionOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   default:
	//     "$ref": "#/responses/error"

	if comment := getCommitComment(ctx); comment != nil {
		changeCommentReaction(ctx, comment, *web.GetForm[*api.EditReactionOption](ctx), false)
	}
}
