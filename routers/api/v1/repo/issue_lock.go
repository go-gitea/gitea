// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
)

// LockIssue lock an issue
func LockIssue(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/issues/{index}/lock issue issueLockIssue
	// ---
	// summary: Lock an issue
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
	//     "$ref": "#/definitions/LockIssueOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	reason := web.GetForm(ctx).(*api.LockIssueOption).Reason
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.APIErrorNotFound(err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		ctx.APIError(http.StatusForbidden, errors.New("no permission to lock this issue"))
		return
	}

	if !issue.IsLocked {
		opt := &issues_model.IssueLockOptions{
			Doer:   ctx.ContextUser,
			Issue:  issue,
			Reason: reason,
		}

		issue.Repo = ctx.Repo.Repository
		err = issues_model.LockIssue(ctx, opt)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}

	ctx.Status(http.StatusNoContent)
}

// UnlockIssue unlock an issue
func UnlockIssue(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/{index}/lock issue issueUnlockIssue
	// ---
	// summary: Unlock an issue
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
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.APIErrorNotFound(err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		ctx.APIError(http.StatusForbidden, errors.New("no permission to unlock this issue"))
		return
	}

	if issue.IsLocked {
		opt := &issues_model.IssueLockOptions{
			Doer:  ctx.ContextUser,
			Issue: issue,
		}

		issue.Repo = ctx.Repo.Repository
		err = issues_model.UnlockIssue(ctx, opt)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}

	ctx.Status(http.StatusNoContent)
}
