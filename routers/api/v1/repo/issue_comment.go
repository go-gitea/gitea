// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package repo

import (
	stdCtx "context"
	"errors"
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/optional"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	issue_service "code.gitea.io/gitea/services/issue"
)

// ListIssueComments list all the comments of an issue
func ListIssueComments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{index}/comments issue issueGetComments
	// ---
	// summary: List all comments on an issue
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
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetRawIssueByIndex", err)
		return
	}
	if !ctx.Repo.CanReadIssuesOrPulls(issue.IsPull) {
		ctx.NotFound()
		return
	}

	issue.Repo = ctx.Repo.Repository

	opts := &issues_model.FindCommentsOptions{
		IssueID: issue.ID,
		Since:   since,
		Before:  before,
		Type:    issues_model.CommentTypeComment,
	}

	comments, err := issues_model.FindComments(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindComments", err)
		return
	}

	totalCount, err := issues_model.CountComments(ctx, opts)
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
		comment.Issue = issue
		apiComments[i] = convert.ToAPIComment(ctx, ctx.Repo.Repository, comments[i])
	}

	ctx.SetTotalCountHeader(totalCount)
	ctx.JSON(http.StatusOK, &apiComments)
}

// ListIssueCommentsAndTimeline list all the comments and events of an issue
func ListIssueCommentsAndTimeline(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{index}/timeline issue issueGetCommentsAndTimeline
	// ---
	// summary: List all comments and events on an issue
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
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetRawIssueByIndex", err)
		return
	}
	issue.Repo = ctx.Repo.Repository

	opts := &issues_model.FindCommentsOptions{
		ListOptions: utils.GetListOptions(ctx),
		IssueID:     issue.ID,
		Since:       since,
		Before:      before,
		Type:        issues_model.CommentTypeUndefined,
	}

	comments, err := issues_model.FindComments(ctx, opts)
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
		if comment.Type != issues_model.CommentTypeCode && isXRefCommentAccessible(ctx, ctx.Doer, comment, issue.RepoID) {
			comment.Issue = issue
			apiComments = append(apiComments, convert.ToTimelineComment(ctx, issue.Repo, comment, ctx.Doer))
		}
	}

	ctx.SetTotalCountHeader(int64(len(apiComments)))
	ctx.JSON(http.StatusOK, &apiComments)
}

func isXRefCommentAccessible(ctx stdCtx.Context, user *user_model.User, c *issues_model.Comment, issueRepoID int64) bool {
	// Remove comments that the user has no permissions to see
	if issues_model.CommentTypeIsRef(c.Type) && c.RefRepoID != issueRepoID && c.RefRepoID != 0 {
		var err error
		// Set RefRepo for description in template
		c.RefRepo, err = repo_model.GetRepositoryByID(ctx, c.RefRepoID)
		if err != nil {
			return false
		}
		perm, err := access_model.GetUserRepoPermission(ctx, c.RefRepo, user)
		if err != nil {
			return false
		}
		if !perm.CanReadIssuesOrPulls(c.RefIsPull) {
			return false
		}
	}
	return true
}

// ListRepoIssueComments returns all issue-comments for a repo
func ListRepoIssueComments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/comments issue issueGetRepoComments
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

	var isPull optional.Option[bool]
	canReadIssue := ctx.Repo.CanRead(unit.TypeIssues)
	canReadPull := ctx.Repo.CanRead(unit.TypePullRequests)
	if canReadIssue && canReadPull {
		isPull = optional.None[bool]()
	} else if canReadIssue {
		isPull = optional.Some(false)
	} else if canReadPull {
		isPull = optional.Some(true)
	} else {
		ctx.NotFound()
		return
	}

	opts := &issues_model.FindCommentsOptions{
		ListOptions: utils.GetListOptions(ctx),
		RepoID:      ctx.Repo.Repository.ID,
		Type:        issues_model.CommentTypeComment,
		Since:       since,
		Before:      before,
		IsPull:      isPull,
	}

	comments, err := issues_model.FindComments(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindComments", err)
		return
	}

	totalCount, err := issues_model.CountComments(ctx, opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	if err = comments.LoadPosters(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadPosters", err)
		return
	}

	apiComments := make([]*api.Comment, len(comments))
	if err := comments.LoadIssues(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssues", err)
		return
	}
	if err := comments.LoadAttachments(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttachments", err)
		return
	}
	if _, err := comments.Issues().LoadRepositories(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadRepositories", err)
		return
	}
	for i := range comments {
		apiComments[i] = convert.ToAPIComment(ctx, ctx.Repo.Repository, comments[i])
	}

	ctx.SetTotalCountHeader(totalCount)
	ctx.JSON(http.StatusOK, &apiComments)
}

// CreateIssueComment create a comment for an issue
func CreateIssueComment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/comments issue issueCreateComment
	// ---
	// summary: Add a comment to an issue
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
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateIssueCommentOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Comment"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	form := web.GetForm(ctx).(*api.CreateIssueCommentOption)
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		return
	}

	if !ctx.Repo.CanReadIssuesOrPulls(issue.IsPull) {
		ctx.NotFound()
		return
	}

	if issue.IsLocked && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) && !ctx.Doer.IsAdmin {
		ctx.Error(http.StatusForbidden, "CreateIssueComment", errors.New(ctx.Locale.TrString("repo.issues.comment_on_locked")))
		return
	}

	comment, err := issue_service.CreateIssueComment(ctx, ctx.Doer, ctx.Repo.Repository, issue, form.Body, nil)
	if err != nil {
		if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.Error(http.StatusForbidden, "CreateIssueComment", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "CreateIssueComment", err)
		}
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToAPIComment(ctx, ctx.Repo.Repository, comment))
}

// GetIssueComment Get a comment by ID
func GetIssueComment(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/comments/{id} issue issueGetComment
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

	comment, err := issues_model.GetCommentByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if issues_model.IsErrCommentNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetCommentByID", err)
		}
		return
	}

	if err = comment.LoadIssue(ctx); err != nil {
		ctx.InternalServerError(err)
		return
	}
	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.Status(http.StatusNotFound)
		return
	}

	if !ctx.Repo.CanReadIssuesOrPulls(comment.Issue.IsPull) {
		ctx.NotFound()
		return
	}

	if comment.Type != issues_model.CommentTypeComment {
		ctx.Status(http.StatusNoContent)
		return
	}

	if err := comment.LoadPoster(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "comment.LoadPoster", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToAPIComment(ctx, ctx.Repo.Repository, comment))
}

// EditIssueComment modify a comment of an issue
func EditIssueComment(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/issues/comments/{id} issue issueEditComment
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
	//     "$ref": "#/definitions/EditIssueCommentOption"
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

	form := web.GetForm(ctx).(*api.EditIssueCommentOption)
	editIssueComment(ctx, *form)
}

// EditIssueCommentDeprecated modify a comment of an issue
func EditIssueCommentDeprecated(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/issues/{index}/comments/{id} issue issueEditCommentDeprecated
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
	//     "$ref": "#/definitions/EditIssueCommentOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Comment"
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	form := web.GetForm(ctx).(*api.EditIssueCommentOption)
	editIssueComment(ctx, *form)
}

func editIssueComment(ctx *context.APIContext, form api.EditIssueCommentOption) {
	comment, err := issues_model.GetCommentByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if issues_model.IsErrCommentNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetCommentByID", err)
		}
		return
	}

	if err := comment.LoadIssue(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
		return
	}

	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.Status(http.StatusNotFound)
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != comment.PosterID && !ctx.Repo.CanWriteIssuesOrPulls(comment.Issue.IsPull)) {
		ctx.Status(http.StatusForbidden)
		return
	}

	if !comment.Type.HasContentSupport() {
		ctx.Status(http.StatusNoContent)
		return
	}

	oldContent := comment.Content
	comment.Content = form.Body
	if err := issue_service.UpdateComment(ctx, comment, comment.ContentVersion, ctx.Doer, oldContent); err != nil {
		if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.Error(http.StatusForbidden, "UpdateComment", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "UpdateComment", err)
		}
		return
	}

	ctx.JSON(http.StatusOK, convert.ToAPIComment(ctx, ctx.Repo.Repository, comment))
}

// DeleteIssueComment delete a comment from an issue
func DeleteIssueComment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/comments/{id} issue issueDeleteComment
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

	deleteIssueComment(ctx)
}

// DeleteIssueCommentDeprecated delete a comment from an issue
func DeleteIssueCommentDeprecated(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/{index}/comments/{id} issue issueDeleteCommentDeprecated
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

	deleteIssueComment(ctx)
}

func deleteIssueComment(ctx *context.APIContext) {
	comment, err := issues_model.GetCommentByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if issues_model.IsErrCommentNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetCommentByID", err)
		}
		return
	}

	if err := comment.LoadIssue(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
		return
	}

	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.Status(http.StatusNotFound)
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != comment.PosterID && !ctx.Repo.CanWriteIssuesOrPulls(comment.Issue.IsPull)) {
		ctx.Status(http.StatusForbidden)
		return
	} else if comment.Type != issues_model.CommentTypeComment {
		ctx.Status(http.StatusNoContent)
		return
	}

	if err = issue_service.DeleteComment(ctx, ctx.Doer, comment); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteCommentByID", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
