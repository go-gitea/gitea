// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"errors"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
)

// PostIssueCommentReaction add a reaction to a comment of a issue
func PostIssueCommentReaction(ctx *context.APIContext, form api.EditReactionOption) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/comments/{id}/reactions issue issuePostCommentReaction
	// ---
	// summary: Add a reaction to a comment of a issue
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
	//   "201":
	//     "$ref": "#/responses/ReactionResponse"
	changeIssueCommentReaction(ctx, form, true)
}

// DeleteIssueCommentReaction list reactions of a issue comment
func DeleteIssueCommentReaction(ctx *context.APIContext, form api.EditReactionOption) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/comments/{id}/reactions issue issueDeleteCommentReaction
	// ---
	// summary: Remove a reaction from a comment of a issue
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
	changeIssueCommentReaction(ctx, form, false)
}

func changeIssueCommentReaction(ctx *context.APIContext, form api.EditReactionOption, isCreateType bool) {
	comment, err := models.GetCommentByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrCommentNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(500, "GetCommentByID", err)
		}
		return
	}

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		ctx.Error(500, "GetIssueByIndex", err)
		return
	}

	if issue.IsLocked && !ctx.Repo.CanWrite(models.UnitTypeIssues) && !ctx.User.IsAdmin {
		ctx.Error(403, "CreateIssueComment", errors.New(ctx.Tr("repo.issues.comment_on_locked")))
		return
	}

	if isCreateType {
		// PostIssueCommentReaction part
		reaction, err := models.CreateCommentReaction(ctx.User, comment.Issue, comment, form.Reaction)
		if err != nil {
			if models.IsErrForbiddenIssueReaction(err) {
				ctx.Error(403, err.Error(), err)
			} else {
				ctx.Error(500, "CreateCommentReaction", err)
			}
			return
		}

		ctx.JSON(201, api.ReactionResponse{
			User:     reaction.User.APIFormat(),
			Reaction: reaction.Type,
			Created:  reaction.CreatedUnix.AsTime(),
		})
	} else {
		// DeleteIssueCommentReaction part
		err = models.DeleteReaction(
			&models.ReactionOptions{
				Type:    form.Reaction,
				Issue:   comment.Issue,
				Comment: comment,
				Doer:    ctx.User,
			})
		if err != nil {
			ctx.Error(500, "DeleteReaction", err)
			return
		}
		ctx.Status(200)
	}
}

// GetIssueCommentReactions list reactions of a issue comment
func GetIssueCommentReactions(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/comments/{id}/reactions issue issueGetCommentReactions
	// ---
	// summary: Get a list reactions of a issue comment
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
	//     "$ref": "#/responses/ReactionResponseList"

	r1 := api.ReactionResponse{
		Reaction: "heart",
		Created:  time.Now(),
		User: &api.User{
			UserName: "aaaaa",
			Language: "not DE",
		},
	}

	r2 := api.ReactionResponse{
		Reaction: "rocket",
		Created:  time.Now(),
		User: &api.User{
			UserName: "user2",
			Language: "not EN",
		},
	}

	var r []api.ReactionResponse
	r = append(r, r1)
	r = append(r, r2)

	ctx.JSON(200, r)
}
