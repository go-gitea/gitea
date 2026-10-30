// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	gocontext "context"
	"errors"
	"net/http"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	user_model "gitea.dev/models/user"
	"gitea.dev/services/context"
	repo_service "gitea.dev/services/repository"
	user_service "gitea.dev/services/user"
)

func canCommentOnCommit(ctx *context.Context) bool {
	return ctx.Doer != nil && !ctx.Repo.Repository.IsArchived
}

func prepareCommitCommentsData(ctx *context.Context, commitSHA string) {
	ctx.Data["CommitID"] = commitSHA
	ctx.Data["DiffNewCommentURL"] = ctx.Repo.RepoLink + "/commit/" + commitSHA + "/comment"
	ctx.Data["CanBlockUser"] = func(blocker, blockee *user_model.User) bool {
		return user_service.CanBlockUser(ctx, ctx.Doer, blocker, blockee)
	}
}

func RenderNewCommitCommentForm(ctx *context.Context) {
	prepareCommitCommentsData(ctx, ctx.PathParam("sha"))
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
	if err := repo_service.CreateCommitComment(ctx, ctx.Doer, ctx.Repo.Repository, commit, treePath, line, content); errors.Is(err, user_model.ErrBlockedUser) {
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
	prepareCommitCommentsData(ctx, commit.ID.String())
	ctx.Data["comments"] = comments
	ctx.HTML(http.StatusOK, tplDiffConversation)
}

func DeleteCommitComment(ctx *context.Context) {
	comment, err := issues_model.GetCommitCommentByID(ctx, ctx.Repo.Repository.ID, ctx.PathParam("sha"), ctx.PathParamInt64("id"))
	if err != nil {
		ctx.ServerError("GetCommitCommentByID", err)
		return
	}
	if comment.PosterID != ctx.Doer.ID && !ctx.Repo.Permission.IsAdmin() {
		ctx.HTTPError(http.StatusForbidden)
		return
	}
	if err := db.WithTx(ctx, func(ctx gocontext.Context) error { return issues_model.DeleteComment(ctx, comment) }); err != nil {
		ctx.ServerError("DeleteComment", err)
		return
	}
	ctx.Status(http.StatusOK)
}
