// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/context"
)

// IssuePinOrUnpin pin or unpin a Issue
func IssuePinOrUnpin(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	// If we don't do this, it will crash when trying to add the pin event to the comment history
	err := issue.LoadRepo(ctx)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		log.Error(err.Error())
		return
	}

	// PinOrUnpin pins or unpins a Issue
	_, err = issues_model.GetIssuePin(ctx, issue)
	if err != nil && !db.IsErrNotExist(err) {
		ctx.Status(http.StatusInternalServerError)
		log.Error(err.Error())
		return
	}

	if db.IsErrNotExist(err) {
		err = issues_model.PinIssue(ctx, issue, ctx.Doer)
	} else {
		err = issues_model.UnpinIssue(ctx, issue, ctx.Doer)
	}

	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		log.Error(err.Error())
		return
	}

	ctx.JSONRedirect(issue.Link())
}

// IssueUnpin unpins a Issue
func IssueUnpin(ctx *context.Context) {
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		log.Error(err.Error())
		return
	}

	// If we don't do this, it will crash when trying to add the pin event to the comment history
	err = issue.LoadRepo(ctx)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		log.Error(err.Error())
		return
	}

	err = issues_model.UnpinIssue(ctx, issue, ctx.Doer)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		log.Error(err.Error())
		return
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
		ctx.Status(http.StatusInternalServerError)
		log.Error(err.Error())
		return
	}

	issue, err := issues_model.GetIssueByID(ctx, form.ID)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		log.Error(err.Error())
		return
	}

	if issue.RepoID != ctx.Repo.Repository.ID {
		ctx.Status(http.StatusNotFound)
		log.Error("Issue does not belong to this repository")
		return
	}

	err = issues_model.MovePin(ctx, issue, form.Position)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		log.Error(err.Error())
		return
	}

	ctx.Status(http.StatusNoContent)
}
