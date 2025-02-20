// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	issue_service "code.gitea.io/gitea/services/issue"
)

// GetIssueCommentReactions list reactions of a comment from an issue
func GetIssueCommentReactions(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/comments/{id}/reactions issue issueGetCommentReactions
	// ---
	// summary: Get a list of reactions from a comment of an issue
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
	// - name: id
	//   in: path
	//   description: id of the comment to edit
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ReactionList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	comment, err := issues_model.GetCommentByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if issues_model.IsErrCommentNotExist(err) {
			ctx.APIErrorNotFound(err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if err := comment.LoadIssue(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.APIErrorNotFound()
		return
	}

	if !ctx.Repo.CanReadIssuesOrPulls(comment.Issue.IsPull) {
		ctx.APIError(http.StatusForbidden, errors.New("no permission to get reactions"))
		return
	}

	reactions, _, err := issues_model.FindCommentReactions(ctx, comment.IssueID, comment.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	_, err = reactions.LoadUsers(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	var result []api.Reaction
	for _, r := range reactions {
		result = append(result, api.Reaction{
			User:     convert.ToUser(ctx, r.User, ctx.Doer),
			Reaction: r.Type,
			Created:  r.CreatedUnix.AsTime(),
		})
	}

	ctx.JSON(http.StatusOK, result)
}

// PostIssueCommentReaction add a reaction to a comment of an issue
func PostIssueCommentReaction(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/comments/{id}/reactions issue issuePostCommentReaction
	// ---
	// summary: Add a reaction to a comment of an issue
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
	// - name: id
	//   in: path
	//   description: id of the comment to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: content
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditReactionOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Reaction"
	//   "201":
	//     "$ref": "#/responses/Reaction"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	form := web.GetForm(ctx).(*api.EditReactionOption)

	changeIssueCommentReaction(ctx, *form, true)
}

// DeleteIssueCommentReaction remove a reaction from a comment of an issue
func DeleteIssueCommentReaction(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/comments/{id}/reactions issue issueDeleteCommentReaction
	// ---
	// summary: Remove a reaction from a comment of an issue
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
	// - name: id
	//   in: path
	//   description: id of the comment to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: content
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditReactionOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	form := web.GetForm(ctx).(*api.EditReactionOption)

	changeIssueCommentReaction(ctx, *form, false)
}

func changeIssueCommentReaction(ctx *context.APIContext, form api.EditReactionOption, isCreateType bool) {
	comment, err := issues_model.GetCommentByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if issues_model.IsErrCommentNotExist(err) {
			ctx.APIErrorNotFound(err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if err = comment.LoadIssue(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.APIErrorNotFound()
		return
	}

	if !ctx.Repo.CanReadIssuesOrPulls(comment.Issue.IsPull) {
		ctx.APIErrorNotFound()
		return
	}

	if comment.Issue.IsLocked && !ctx.Repo.CanWriteIssuesOrPulls(comment.Issue.IsPull) {
		ctx.APIError(http.StatusForbidden, errors.New("no permission to change reaction"))
		return
	}

	if isCreateType {
		// PostIssueCommentReaction part
		reaction, err := issue_service.CreateCommentReaction(ctx, ctx.Doer, comment, form.Reaction)
		if err != nil {
			if issues_model.IsErrForbiddenIssueReaction(err) || errors.Is(err, user_model.ErrBlockedUser) {
				ctx.APIError(http.StatusForbidden, err)
			} else if issues_model.IsErrReactionAlreadyExist(err) {
				ctx.JSON(http.StatusOK, api.Reaction{
					User:     convert.ToUser(ctx, ctx.Doer, ctx.Doer),
					Reaction: reaction.Type,
					Created:  reaction.CreatedUnix.AsTime(),
				})
			} else {
				ctx.APIErrorInternal(err)
			}
			return
		}

		ctx.JSON(http.StatusCreated, api.Reaction{
			User:     convert.ToUser(ctx, ctx.Doer, ctx.Doer),
			Reaction: reaction.Type,
			Created:  reaction.CreatedUnix.AsTime(),
		})
	} else {
		// DeleteIssueCommentReaction part
		err = issues_model.DeleteCommentReaction(ctx, ctx.Doer.ID, comment.Issue.ID, comment.ID, form.Reaction)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		// ToDo respond 204
		ctx.Status(http.StatusOK)
	}
}

// GetIssueReactions list reactions of an issue
func GetIssueReactions(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{index}/reactions issue issueGetIssueReactions
	// ---
	// summary: Get a list reactions of an issue
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
	// - name: index
	//   in: path
	//   description: index of the issue
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
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	issue, err := issues_model.GetIssueWithAttrsByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if !ctx.Repo.CanReadIssuesOrPulls(issue.IsPull) {
		ctx.APIError(http.StatusForbidden, errors.New("no permission to get reactions"))
		return
	}

	reactions, count, err := issues_model.FindIssueReactions(ctx, issue.ID, utils.GetListOptions(ctx))
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	_, err = reactions.LoadUsers(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	var result []api.Reaction
	for _, r := range reactions {
		result = append(result, api.Reaction{
			User:     convert.ToUser(ctx, r.User, ctx.Doer),
			Reaction: r.Type,
			Created:  r.CreatedUnix.AsTime(),
		})
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, result)
}

// PostIssueReaction add a reaction to an issue
func PostIssueReaction(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/reactions issue issuePostIssueReaction
	// ---
	// summary: Add a reaction to an issue
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
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// - name: content
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditReactionOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Reaction"
	//   "201":
	//     "$ref": "#/responses/Reaction"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	form := web.GetForm(ctx).(*api.EditReactionOption)
	changeIssueReaction(ctx, *form, true)
}

// DeleteIssueReaction remove a reaction from an issue
func DeleteIssueReaction(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/{index}/reactions issue issueDeleteIssueReaction
	// ---
	// summary: Remove a reaction from an issue
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
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// - name: content
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditReactionOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	form := web.GetForm(ctx).(*api.EditReactionOption)
	changeIssueReaction(ctx, *form, false)
}

func changeIssueReaction(ctx *context.APIContext, form api.EditReactionOption, isCreateType bool) {
	issue, err := issues_model.GetIssueWithAttrsByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if issue.IsLocked && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		ctx.APIError(http.StatusForbidden, errors.New("no permission to change reaction"))
		return
	}

	if isCreateType {
		// PostIssueReaction part
		reaction, err := issue_service.CreateIssueReaction(ctx, ctx.Doer, issue, form.Reaction)
		if err != nil {
			if issues_model.IsErrForbiddenIssueReaction(err) || errors.Is(err, user_model.ErrBlockedUser) {
				ctx.APIError(http.StatusForbidden, err)
			} else if issues_model.IsErrReactionAlreadyExist(err) {
				ctx.JSON(http.StatusOK, api.Reaction{
					User:     convert.ToUser(ctx, ctx.Doer, ctx.Doer),
					Reaction: reaction.Type,
					Created:  reaction.CreatedUnix.AsTime(),
				})
			} else {
				ctx.APIErrorInternal(err)
			}
			return
		}

		ctx.JSON(http.StatusCreated, api.Reaction{
			User:     convert.ToUser(ctx, ctx.Doer, ctx.Doer),
			Reaction: reaction.Type,
			Created:  reaction.CreatedUnix.AsTime(),
		})
	} else {
		// DeleteIssueReaction part
		err = issues_model.DeleteIssueReaction(ctx, ctx.Doer.ID, issue.ID, form.Reaction)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		// ToDo respond 204
		ctx.Status(http.StatusOK)
	}
}
