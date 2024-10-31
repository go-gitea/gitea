// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	conversations_model "code.gitea.io/gitea/models/conversations"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	conversation_service "code.gitea.io/gitea/services/conversation"
	"code.gitea.io/gitea/services/convert"
)

// ListConversationComments list all the comments of an conversation
func ListConversationComments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/conversations/{index}/comments conversation conversationGetComments
	// ---
	// summary: List all comments on an conversation
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
	//   description: index of the conversation
	//   type: integer
	//   format: int64
	//   required: true
	// - name: since
	//   in: query
	//   description: if provided, only comments updated since the specified time are returned.
	//   type: string
	//   format: date-time
	// - name: before
	//   in: query
	//   description: if provided, only comments updated before the provided time are returned.
	//   type: string
	//   format: date-time
	// responses:
	//   "200":
	//     "$ref": "#/responses/CommentList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	before, since, err := context.GetQueryBeforeSince(ctx.Base)
	if err != nil {
		ctx.Error(http.StatusUnprocessableEntity, "GetQueryBeforeSince", err)
		return
	}
	conversation, err := conversations_model.GetConversationByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64(":index"))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetRawConversationByIndex", err)
		return
	}
	if !ctx.Repo.CanReadConversations() {
		ctx.NotFound()
		return
	}

	conversation.Repo = ctx.Repo.Repository

	opts := &conversations_model.FindCommentsOptions{
		ConversationID: conversation.ID,
		Since:          since,
		Before:         before,
		Type:           conversations_model.CommentTypeComment,
	}

	comments, err := conversations_model.FindComments(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindComments", err)
		return
	}

	totalCount, err := conversations_model.CountComments(ctx, opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	if err := comments.LoadPosters(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadPosters", err)
		return
	}

	if err := comments.LoadAttachments(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttachments", err)
		return
	}

	apiComments := make([]*api.Comment, len(comments))
	for i, comment := range comments {
		comment.Conversation = conversation
		apiComments[i] = convert.ConversationToAPIComment(ctx, ctx.Repo.Repository, comments[i])
	}

	ctx.SetTotalCountHeader(totalCount)
	ctx.JSON(http.StatusOK, &apiComments)
}

// ListConversationCommentsAndTimeline list all the comments and events of an conversation
func ListConversationCommentsAndTimeline(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/conversations/{index}/timeline conversation conversationGetCommentsAndTimeline
	// ---
	// summary: List all comments and events on an conversation
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
	//   description: index of the conversation
	//   type: integer
	//   format: int64
	//   required: true
	// - name: since
	//   in: query
	//   description: if provided, only comments updated since the specified time are returned.
	//   type: string
	//   format: date-time
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// - name: before
	//   in: query
	//   description: if provided, only comments updated before the provided time are returned.
	//   type: string
	//   format: date-time
	// responses:
	//   "200":
	//     "$ref": "#/responses/TimelineList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	before, since, err := context.GetQueryBeforeSince(ctx.Base)
	if err != nil {
		ctx.Error(http.StatusUnprocessableEntity, "GetQueryBeforeSince", err)
		return
	}
	conversation, err := conversations_model.GetConversationByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64(":index"))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetRawConversationByIndex", err)
		return
	}
	conversation.Repo = ctx.Repo.Repository

	opts := &conversations_model.FindCommentsOptions{
		ListOptions:    utils.GetListOptions(ctx),
		ConversationID: conversation.ID,
		Since:          since,
		Before:         before,
		Type:           conversations_model.CommentTypeUndefined,
	}

	comments, err := conversations_model.FindComments(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindComments", err)
		return
	}

	if err := comments.LoadPosters(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadPosters", err)
		return
	}

	var apiComments []*api.TimelineComment
	for _, comment := range comments {
		comment.Conversation = conversation
		apiComments = append(apiComments, convert.ConversationCommentToTimelineComment(ctx, conversation.Repo, comment, ctx.Doer))
	}

	ctx.SetTotalCountHeader(int64(len(apiComments)))
	ctx.JSON(http.StatusOK, &apiComments)
}

// ListRepoConversationComments returns all conversation-comments for a repo
func ListRepoConversationComments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/conversations/comments conversation conversationGetRepoComments
	// ---
	// summary: List all comments in a repository
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
	// - name: since
	//   in: query
	//   description: if provided, only comments updated since the provided time are returned.
	//   type: string
	//   format: date-time
	// - name: before
	//   in: query
	//   description: if provided, only comments updated before the provided time are returned.
	//   type: string
	//   format: date-time
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
	//     "$ref": "#/responses/CommentList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	before, since, err := context.GetQueryBeforeSince(ctx.Base)
	if err != nil {
		ctx.Error(http.StatusUnprocessableEntity, "GetQueryBeforeSince", err)
		return
	}

	canReadConversation := ctx.Repo.CanRead(unit.TypeConversations)
	if !canReadConversation {
		ctx.NotFound()
		return
	}

	opts := &conversations_model.FindCommentsOptions{
		ListOptions: utils.GetListOptions(ctx),
		RepoID:      ctx.Repo.Repository.ID,
		Type:        conversations_model.CommentTypeComment,
		Since:       since,
		Before:      before,
	}

	comments, err := conversations_model.FindComments(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindComments", err)
		return
	}

	totalCount, err := conversations_model.CountComments(ctx, opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	if err = comments.LoadPosters(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadPosters", err)
		return
	}

	apiComments := make([]*api.Comment, len(comments))
	if err := comments.LoadConversations(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadConversations", err)
		return
	}
	if err := comments.LoadAttachments(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttachments", err)
		return
	}
	if _, err := comments.Conversations().LoadRepositories(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadRepositories", err)
		return
	}
	for i := range comments {
		apiComments[i] = convert.ConversationToAPIComment(ctx, ctx.Repo.Repository, comments[i])
	}

	ctx.SetTotalCountHeader(totalCount)
	ctx.JSON(http.StatusOK, &apiComments)
}

// CreateConversationComment create a comment for an conversation
func CreateConversationComment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/conversations/{index}/comments conversation conversationCreateComment
	// ---
	// summary: Add a comment to an conversation
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
	//   description: index of the conversation
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateConversationCommentOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Comment"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	form := web.GetForm(ctx).(*api.CreateConversationCommentOption)
	conversation, err := conversations_model.GetConversationByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64(":index"))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetConversationByIndex", err)
		return
	}

	if !ctx.Repo.CanReadConversations() {
		ctx.NotFound()
		return
	}

	if conversation.IsLocked && !ctx.Repo.CanWriteConversations() && !ctx.Doer.IsAdmin {
		ctx.Error(http.StatusForbidden, "CreateConversationComment", errors.New(ctx.Locale.TrString("repo.conversations.comment_on_locked")))
		return
	}

	comment, err := conversation_service.CreateConversationComment(ctx, ctx.Doer, ctx.Repo.Repository, conversation, form.Body, nil)
	if err != nil {
		if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.Error(http.StatusForbidden, "CreateConversationComment", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "CreateConversationComment", err)
		}
		return
	}

	ctx.JSON(http.StatusCreated, convert.ConversationToAPIComment(ctx, ctx.Repo.Repository, comment))
}

// GetConversationComment Get a comment by ID
func GetConversationComment(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/conversations/comments/{id} conversation conversationGetComment
	// ---
	// summary: Get a comment
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
	//   description: id of the comment
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Comment"
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	comment, err := conversations_model.GetCommentByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		if conversations_model.IsErrCommentNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetCommentByID", err)
		}
		return
	}

	if err = comment.LoadConversation(ctx); err != nil {
		ctx.InternalServerError(err)
		return
	}
	if comment.Conversation.RepoID != ctx.Repo.Repository.ID {
		ctx.Status(http.StatusNotFound)
		return
	}

	if !ctx.Repo.CanReadConversations() {
		ctx.NotFound()
		return
	}

	if comment.Type != conversations_model.CommentTypeComment {
		ctx.Status(http.StatusNoContent)
		return
	}

	if err := comment.LoadPoster(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "comment.LoadPoster", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ConversationToAPIComment(ctx, ctx.Repo.Repository, comment))
}

// EditConversationComment modify a comment of an conversation
func EditConversationComment(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/conversations/comments/{id} conversation conversationEditComment
	// ---
	// summary: Edit a comment
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
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditConversationCommentOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Comment"
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	form := web.GetForm(ctx).(*api.EditConversationCommentOption)
	editConversationComment(ctx, *form)
}

// EditConversationCommentDeprecated modify a comment of an conversation
func EditConversationCommentDeprecated(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/conversations/{index}/comments/{id} conversation conversationEditCommentDeprecated
	// ---
	// summary: Edit a comment
	// deprecated: true
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
	//   description: this parameter is ignored
	//   type: integer
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the comment to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditConversationCommentOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Comment"
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	form := web.GetForm(ctx).(*api.EditConversationCommentOption)
	editConversationComment(ctx, *form)
}

func editConversationComment(ctx *context.APIContext, form api.EditConversationCommentOption) {
	comment, err := conversations_model.GetCommentByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		if conversations_model.IsErrCommentNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetCommentByID", err)
		}
		return
	}

	if err := comment.LoadConversation(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadConversation", err)
		return
	}

	if comment.Conversation.RepoID != ctx.Repo.Repository.ID {
		ctx.Status(http.StatusNotFound)
		return
	}

	if !ctx.IsSigned {
		ctx.Status(http.StatusForbidden)
		return
	}

	if !comment.Type.HasContentSupport() {
		ctx.Status(http.StatusNoContent)
		return
	}

	oldContent := comment.Content
	comment.Content = form.Body
	if err := conversation_service.UpdateComment(ctx, comment, comment.ContentVersion, ctx.Doer, oldContent); err != nil {
		if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.Error(http.StatusForbidden, "UpdateComment", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "UpdateComment", err)
		}
		return
	}

	ctx.JSON(http.StatusOK, convert.ConversationToAPIComment(ctx, ctx.Repo.Repository, comment))
}

// DeleteConversationComment delete a comment from an conversation
func DeleteConversationComment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/conversations/comments/{id} conversation conversationDeleteComment
	// ---
	// summary: Delete a comment
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

	deleteConversationComment(ctx)
}

// DeleteConversationCommentDeprecated delete a comment from an conversation
func DeleteConversationCommentDeprecated(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/conversations/{index}/comments/{id} conversation conversationDeleteCommentDeprecated
	// ---
	// summary: Delete a comment
	// deprecated: true
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
	//   description: this parameter is ignored
	//   type: integer
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

	deleteConversationComment(ctx)
}

func deleteConversationComment(ctx *context.APIContext) {
	comment, err := conversations_model.GetCommentByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		if conversations_model.IsErrCommentNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetCommentByID", err)
		}
		return
	}

	if err := comment.LoadConversation(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadConversation", err)
		return
	}

	if comment.Conversation.RepoID != ctx.Repo.Repository.ID {
		ctx.Status(http.StatusNotFound)
		return
	}

	if !ctx.IsSigned {
		ctx.Status(http.StatusForbidden)
		return
	} else if comment.Type != conversations_model.CommentTypeComment {
		ctx.Status(http.StatusNoContent)
		return
	}

	if err = conversation_service.DeleteComment(ctx, ctx.Doer, comment); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteCommentByID", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
