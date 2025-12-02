// Copyright 2024 The Gitea Authors.
// SPDX-License-Identifier: MIT

package shared

import (
	"errors"
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	user_service "code.gitea.io/gitea/services/user"
)

func ListBlocks(ctx *context.APIContext, blocker *user_model.User) {
	blocks, total, err := user_model.FindBlockings(ctx, &user_model.FindBlockingOptions{
		ListOptions: utils.GetListOptions(ctx),
		BlockerID:   blocker.ID,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if err := user_model.BlockingList(blocks).LoadAttributes(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	users := make([]*api.User, 0, len(blocks))
	for _, b := range blocks {
		users = append(users, convert.ToUser(ctx, b.Blockee, blocker))
	}

	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, &users)
}

func CheckUserBlock(ctx *context.APIContext, blocker *user_model.User) {
	blockee, err := user_model.GetUserByName(ctx, ctx.PathParam("username"))
	if err != nil {
		ctx.APIErrorNotFound("GetUserByName", err)
		return
	}

	status := http.StatusNotFound
	blocking, err := user_model.GetBlocking(ctx, blocker.ID, blockee.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if blocking != nil {
		status = http.StatusNoContent
	}

	ctx.Status(status)
}

func BlockUser(ctx *context.APIContext, blocker *user_model.User) {
	blockee, err := user_model.GetUserByName(ctx, ctx.PathParam("username"))
	if err != nil {
		ctx.APIErrorNotFound("GetUserByName", err)
		return
	}

	if err := user_service.BlockUser(ctx, ctx.Doer, blocker, blockee, ctx.FormString("note")); err != nil {
		if errors.Is(err, user_model.ErrCanNotBlock) || errors.Is(err, user_model.ErrBlockOrganization) {
			ctx.APIError(http.StatusBadRequest, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

func UnblockUser(ctx *context.APIContext, doer, blocker *user_model.User) {
	blockee, err := user_model.GetUserByName(ctx, ctx.PathParam("username"))
	if err != nil {
		ctx.APIErrorNotFound("GetUserByName", err)
		return
	}

	if err := user_service.UnblockUser(ctx, doer, blocker, blockee); err != nil {
		if errors.Is(err, user_model.ErrCanNotUnblock) || errors.Is(err, user_model.ErrBlockOrganization) {
			ctx.APIError(http.StatusBadRequest, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}
