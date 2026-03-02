// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/services/context"
	repo_service "code.gitea.io/gitea/services/repository"
)

func acceptReparent(ctx *context.Context) {
	repoTransfer, err := repo_model.GetPendingRepositoryTransfer(ctx, ctx.Repo.Repository)
	if err != nil {
		if repo_model.IsErrNoPendingTransfer(err) || errors.Is(err, repo_model.ErrNoPendingRepoTransfer{}) {
			ctx.NotFound(err)
			return
		}
		ctx.ServerError("GetPendingRepositoryTransfer", err)
		return
	}

	if !repoTransfer.CanUserAcceptOrRejectTransfer(ctx, ctx.Doer) {
		ctx.HTTPError(http.StatusForbidden, "Only the repository owner can accept reparenting")
		return
	}

	err = repo_service.AcceptReparent(ctx, ctx.Doer, ctx.Repo.Repository)
	if err == nil {
		ctx.Flash.Success(ctx.Tr("repo.reparent.success"))
		ctx.Redirect(ctx.Repo.Repository.Link())
		return
	}
	handleActionError(ctx, err)
}

func rejectReparent(ctx *context.Context) {
	repoTransfer, err := repo_model.GetPendingRepositoryTransfer(ctx, ctx.Repo.Repository)
	if err != nil {
		if repo_model.IsErrNoPendingTransfer(err) || errors.Is(err, repo_model.ErrNoPendingRepoTransfer{}) {
			ctx.NotFound(err)
			return
		}
		ctx.ServerError("GetPendingRepositoryTransfer", err)
		return
	}

	if !repoTransfer.CanUserAcceptOrRejectTransfer(ctx, ctx.Doer) && repoTransfer.DoerID != ctx.Doer.ID {
		ctx.HTTPError(http.StatusForbidden, "Only the repository owner or the initiator can reject/cancel reparenting")
		return
	}

	err = repo_service.CancelRepositoryReparent(ctx, ctx.Doer, ctx.Repo.Repository)
	if err == nil {
		ctx.Flash.Success(ctx.Tr("repo.reparent.rejected"))
		ctx.Redirect(ctx.Repo.Repository.Link())
		return
	}
	handleActionError(ctx, err)
}

func ActionReparent(ctx *context.Context) {
	switch ctx.PathParam("action") {
	case "accept_reparent":
		acceptReparent(ctx)
	case "reject_reparent":
		rejectReparent(ctx)
	}
}
