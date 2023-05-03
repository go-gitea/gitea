// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
)

// IssuePinOrUnpin pin or unpin a Issue
func IssuePinOrUnpin(ctx *context.Context) {
	issue := GetActionIssue(ctx)

	// If we don't do this, it will crash when trying to add the pin event to the comment history
	err := issue.LoadRepo(ctx)
	if err != nil {
		ctx.ServerError("LoadRepo", err)
		return
	}

	err = issue.PinOrUnpin(ctx, ctx.ContextUser)
	if err != nil {
		ctx.ServerError("PinOrUnpinIssue", err)
		return
	}

	ctx.Redirect(issue.Link())
}

// IssueUnpin unpins a Issue
func IssueUnpin(ctx *context.Context) {
	issue, err := issues_model.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	// If we don't do this, it will crash when trying to add the pin event to the comment history
	err = issue.LoadRepo(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	err = issue.Unpin(ctx, ctx.ContextUser)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
	}

	ctx.Status(http.StatusNoContent)
}

// IssuePinMove moves a pinned Issue
func IssuePinMove(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, "Only signed in users are allowed to perform this action.")
		return
	}

	type movePinIssueForm struct {
		ID       int64 `json:"id"`
		Position int   `json:"position"`
	}

	form := &movePinIssueForm{}
	if err := json.NewDecoder(ctx.Req.Body).Decode(&form); err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
	}

	issue, err := issues_model.GetIssueByID(ctx, form.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
	}

	err = issue.MovePin(form.Position)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
	}
}
