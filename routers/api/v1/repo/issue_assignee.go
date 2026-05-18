// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	issue_service "code.gitea.io/gitea/services/issue"
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if !ctx.Repo.Permission.CanReadIssuesOrPulls(issue.IsPull) {
		ctx.APIErrorNotFound()
		return
	}

	assignee, err := user_model.GetUserByName(ctx, ctx.PathParam("assignee"))
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	canAssign, err := access_model.CanBeAssigned(ctx, assignee, ctx.Repo.Repository)
	if err != nil {
		if errors.Is(err, access_model.ErrOrganizationNotAssignee) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	if !canAssign {
		ctx.APIErrorNotFound()
		return
	}

	ctx.Status(http.StatusNoContent)
}

func updateIssueAssignees(ctx *context.APIContext, opts api.IssueAssigneesOption, isAdd bool) {
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
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
		var userNotExistErr user_model.ErrUserNotExist
		if errors.As(err, &userNotExistErr) {
			ctx.APIError(http.StatusUnprocessableEntity, fmt.Sprintf("Assignee does not exist: [name: %s]", userNotExistErr.Name))
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if isAdd {
		err = issue_service.AddAssignees(ctx, issue, ctx.Doer, assigneeIDs)
	} else {
		err = issue_service.RemoveAssignees(ctx, issue, ctx.Doer, assigneeIDs)
	}

	switch {
	case errors.Is(err, user_model.ErrBlockedUser):
		ctx.APIError(http.StatusForbidden, err)
		return
	case errors.Is(err, access_model.ErrOrganizationNotAssignee):
		ctx.APIError(http.StatusUnprocessableEntity, err)
		return
	case repo_model.IsErrUserDoesNotHaveAccessToRepo(err):
		ctx.APIError(http.StatusUnprocessableEntity, err)
		return
	case err != nil:
		ctx.APIErrorInternal(err)
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
