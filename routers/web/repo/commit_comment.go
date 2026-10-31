// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	issues_model "gitea.dev/models/issues"
	user_model "gitea.dev/models/user"
	"gitea.dev/services/context"
	issue_service "gitea.dev/services/issue"
)

func RenderNewCommitCommentForm(ctx *context.Context) {
	prepareCommitCommentTemplateData(ctx, ctx.PathParam("sha"))
	ctx.HTML(http.StatusOK, tplNewComment)
}

func CreateCommitComment(ctx *context.Context) {
	content := ctx.FormString("content")
	treePath := ctx.FormString("path")
	side := ctx.FormString("side")
	line := ctx.FormInt64("line")
	if content == "" || treePath == "" || line <= 0 || (side != "previous" && side != "proposed") {
		ctx.HTTPError(http.StatusBadRequest)
		return
	}
	if side == "previous" {
		line = -line
	}

	commit, err := ctx.Repo.GitRepo.GetCommit(ctx, ctx.PathParam("sha"))
	if err != nil {
		ctx.ServerError("GetCommit", err)
		return
	}
	if err := issue_service.CreateCommitComment(ctx, ctx.Doer, ctx.Repo.Repository, commit, treePath, line, content, ctx.FormStrings("files")); errors.Is(err, user_model.ErrBlockedUser) {
		ctx.JSONError(ctx.Tr("repo.issues.comment.blocked_user"))
		return
	} else if err != nil {
		ctx.ServerError("CreateCommitComment", err)
		return
	}

	comments, err := issues_model.FetchCommitCodeCommentsByLine(ctx, ctx.Repo.Repository, commit.ID.String(), treePath, line)
	if err != nil {
		ctx.ServerError("FetchCommitCodeCommentsByLine", err)
		return
	}
	prepareCommitCommentTemplateData(ctx, commit.ID.String())
	ctx.Data["comments"] = comments
	ctx.HTML(http.StatusOK, tplDiffConversation)
}
