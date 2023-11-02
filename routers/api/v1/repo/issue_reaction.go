// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/convert"
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

	comment, err := issues_model.GetCommentByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if issues_model.IsErrCommentNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetCommentByID", err)
		}
		return
	}

	if err := comment.LoadIssue(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "comment.LoadIssue", err)
		return
	}

	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound()
		return
	}

	if !ctx.Repo.CanReadIssuesOrPulls(comment.Issue.IsPull) {
		ctx.Error(http.StatusForbidden, "GetIssueCommentReactions", errors.New("no permission to get reactions"))
		return
	}

	reactions, _, err := issues_model.FindCommentReactions(comment.IssueID, comment.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindCommentReactions", err)
		return
	}
	_, err = reactions.LoadUsers(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ReactionList.LoadUsers()", err)
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

	form := web.GetForm(ctx).(*api.EditReactionOption)

	changeIssueCommentReaction(ctx, *form, false)
}

func changeIssueCommentReaction(ctx *context.APIContext, form api.EditReactionOption, isCreateType bool) {
	comment, err := issues_model.GetCommentByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if issues_model.IsErrCommentNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetCommentByID", err)
		}
		return
	}

	if err = comment.LoadIssue(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "comment.LoadIssue() failed", err)
		return
	}

	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound()
		return
	}

	if !ctx.Repo.CanReadIssuesOrPulls(comment.Issue.IsPull) {
		ctx.NotFound()
		return
	}

	if comment.Issue.IsLocked && !ctx.Repo.CanWriteIssuesOrPulls(comment.Issue.IsPull) {
		ctx.Error(http.StatusForbidden, "ChangeIssueCommentReaction", errors.New("no permission to change reaction"))
		return
	}

	if isCreateType {
		// PostIssueCommentReaction part
		reaction, err := issues_model.CreateCommentReaction(ctx.Doer.ID, comment.Issue.ID, comment.ID, form.Reaction)
		if err != nil {
			if issues_model.IsErrForbiddenIssueReaction(err) {
				ctx.Error(http.StatusForbidden, err.Error(), err)
			} else if issues_model.IsErrReactionAlreadyExist(err) {
				ctx.JSON(http.StatusOK, api.Reaction{
					User:     convert.ToUser(ctx, ctx.Doer, ctx.Doer),
					Reaction: reaction.Type,
					Created:  reaction.CreatedUnix.AsTime(),
				})
			} else {
				ctx.Error(http.StatusInternalServerError, "CreateCommentReaction", err)
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
		err = issues_model.DeleteCommentReaction(ctx.Doer.ID, comment.Issue.ID, comment.ID, form.Reaction)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "DeleteCommentReaction", err)
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

	issue, err := issues_model.GetIssueWithAttrsByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	if !ctx.Repo.CanReadIssuesOrPulls(issue.IsPull) {
		ctx.Error(http.StatusForbidden, "GetIssueReactions", errors.New("no permission to get reactions"))
		return
	}

	reactions, count, err := issues_model.FindIssueReactions(issue.ID, utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindIssueReactions", err)
		return
	}
	_, err = reactions.LoadUsers(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ReactionList.LoadUsers()", err)
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
	form := web.GetForm(ctx).(*api.EditReactionOption)
	changeIssueReaction(ctx, *form, false)
}

func changeIssueReaction(ctx *context.APIContext, form api.EditReactionOption, isCreateType bool) {
	issue, err := issues_model.GetIssueWithAttrsByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	if issue.IsLocked && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		ctx.Error(http.StatusForbidden, "ChangeIssueCommentReaction", errors.New("no permission to change reaction"))
		return
	}

	if isCreateType {
		// PostIssueReaction part
		reaction, err := issues_model.CreateIssueReaction(ctx.Doer.ID, issue.ID, form.Reaction)
		if err != nil {
			if issues_model.IsErrForbiddenIssueReaction(err) {
				ctx.Error(http.StatusForbidden, err.Error(), err)
			} else if issues_model.IsErrReactionAlreadyExist(err) {
				ctx.JSON(http.StatusOK, api.Reaction{
					User:     convert.ToUser(ctx, ctx.Doer, ctx.Doer),
					Reaction: reaction.Type,
					Created:  reaction.CreatedUnix.AsTime(),
				})
			} else {
				ctx.Error(http.StatusInternalServerError, "CreateCommentReaction", err)
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
		err = issues_model.DeleteIssueReaction(ctx.Doer.ID, issue.ID, form.Reaction)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "DeleteIssueReaction", err)
			return
		}
		// ToDo respond 204
		ctx.Status(http.StatusOK)
	}
}
