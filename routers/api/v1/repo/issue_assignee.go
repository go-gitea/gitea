// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	issue_service "gitea.dev/services/issue"
)

// AddIssueAssignees add assignees to an issue
func AddIssueAssignees(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/assignees issue issueAddAssignees
	// ---
	// summary: Add assignees to an issue
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
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/IssueAssigneesOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Issue"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	opts := web.GetForm(ctx).(*api.IssueAssigneesOption)
	updateIssueAssignees(ctx, *opts, true)
}

// DeleteIssueAssignees remove assignees from an issue
func DeleteIssueAssignees(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/{index}/assignees issue issueRemoveAssignees
	// ---
	// summary: Remove assignees from an issue
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
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/IssueAssigneesOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Issue"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	opts := web.GetForm(ctx).(*api.IssueAssigneesOption)
	updateIssueAssignees(ctx, *opts, false)
}

// CheckIssueAssignee check if a user can be assigned to an issue
func CheckIssueAssignee(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{index}/assignees/{assignee} issue issueCheckAssignee
	// ---
	// summary: Check if a user can be assigned to an issue
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
	// - name: assignee
	//   in: path
	//   description: username of the user to check for being an assignee
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	if !ctx.Repo.Permission.CanReadIssuesOrPulls(issue.IsPull) {
		ctx.APIErrorNotFound()
		return
	}

	if checkAssignableUser(ctx, ctx.PathParam("assignee"), ctx.Repo.Repository) {
		ctx.Status(http.StatusNoContent)
	}
}

// checkAssignableUser resolves assigneeName and verifies the user can be assigned to issues in repo.
// Returns true only when the user resolves AND is assignable; the caller is responsible for writing the 204.
// On any failure it writes the appropriate API response and returns false.
func checkAssignableUser(ctx *context.APIContext, assigneeName string, repo *repo_model.Repository) bool {
	assignee, err := user_model.GetUserByName(ctx, assigneeName)
	if err != nil {
		ctx.APIErrorAuto(err)
		return false
	}

	canAssign, err := access_model.CanBeAssigned(ctx, assignee, repo)
	if err != nil {
		ctx.APIErrorAuto(err)
		return false
	}

	if !canAssign {
		ctx.APIErrorNotFound()
		return false
	}

	return true
}

func updateIssueAssignees(ctx *context.APIContext, opts api.IssueAssigneesOption, isAdd bool) {
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	if !ctx.Repo.Permission.CanWriteIssuesOrPulls(issue.IsPull) {
		ctx.Status(http.StatusForbidden)
		return
	}

	if err := issue.LoadAttributes(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	assigneeIDs, err := user_model.GetUserIDsByNames(ctx, opts.Assignees, false)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err.Error())
			return
		}
		ctx.APIErrorAuto(err)
		return
	}

	if isAdd {
		err = issue_service.AddAssignees(ctx, issue, ctx.Doer, assigneeIDs)
	} else {
		err = issue_service.RemoveAssignees(ctx, issue, ctx.Doer, assigneeIDs)
	}

	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	issue, err = issues_model.GetIssueByID(ctx, issue.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	status := http.StatusOK
	if isAdd {
		status = http.StatusCreated
	}
	ctx.JSON(status, convert.ToAPIIssue(ctx, ctx.Doer, issue))
}
