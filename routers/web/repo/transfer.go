// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"code.gitea.io/gitea/services/context"
	repo_service "code.gitea.io/gitea/services/repository"
)

func acceptTransfer(ctx *context.Context) {
	err := repo_service.AcceptTransferOwnership(ctx, ctx.Repo.Repository, ctx.Doer)
	if err == nil {
		ctx.Flash.Success(ctx.Tr("repo.settings.transfer.success"))
		ctx.Redirect(ctx.Repo.Repository.Link())
		return
	}
	handleActionError(ctx, err)
}

func rejectTransfer(ctx *context.Context) {
	err := repo_service.RejectRepositoryTransfer(ctx, ctx.Repo.Repository, ctx.Doer)
	if err == nil {
		ctx.Flash.Success(ctx.Tr("repo.settings.transfer.rejected"))
		ctx.Redirect(ctx.Repo.Repository.Link())
		return
	}
	handleActionError(ctx, err)
}

func ActionTransfer(ctx *context.Context) {
	switch ctx.PathParam("action") {
	case "accept_transfer":
		acceptTransfer(ctx)
	case "reject_transfer":
		rejectTransfer(ctx)
	}
}
