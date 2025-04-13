// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"errors"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	user_service "code.gitea.io/gitea/services/user"
)

func BlockedUsers(ctx *context.Context, blocker *user_model.User) {
	blocks, _, err := user_model.FindBlockings(ctx, &user_model.FindBlockingOptions{
		BlockerID: blocker.ID,
	})
	if err != nil {
		ctx.ServerError("FindBlockings", err)
		return
	}
	if err := user_model.BlockingList(blocks).LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}
	ctx.Data["UserBlocks"] = blocks
}

func BlockedUsersPost(ctx *context.Context, blocker *user_model.User) {
	form := web.GetForm(ctx).(*forms.BlockUserForm)
	if ctx.HasError() {
		ctx.ServerError("FormValidation", nil)
		return
	}

	blockee, err := user_model.GetUserByName(ctx, form.Blockee)
	if err != nil {
		ctx.ServerError("GetUserByName", nil)
		return
	}

	switch form.Action {
	case "block":
		if err := user_service.BlockUser(ctx, ctx.Doer, blocker, blockee, form.Note); err != nil {
			if errors.Is(err, user_model.ErrCanNotBlock) || errors.Is(err, user_model.ErrBlockOrganization) {
				ctx.Flash.Error(ctx.Tr("user.block.block.failure", err.Error()))
			} else {
				ctx.ServerError("BlockUser", err)
				return
			}
		}
	case "unblock":
		if err := user_service.UnblockUser(ctx, ctx.Doer, blocker, blockee); err != nil {
			if errors.Is(err, user_model.ErrCanNotUnblock) || errors.Is(err, user_model.ErrBlockOrganization) {
				ctx.Flash.Error(ctx.Tr("user.block.unblock.failure", err.Error()))
			} else {
				ctx.ServerError("UnblockUser", err)
				return
			}
		}
	case "note":
		block, err := user_model.GetBlocking(ctx, blocker.ID, blockee.ID)
		if err != nil {
			ctx.ServerError("GetBlocking", err)
			return
		}
		if block != nil {
			if err := user_model.UpdateBlockingNote(ctx, block.ID, form.Note); err != nil {
				ctx.ServerError("UpdateBlockingNote", err)
				return
			}
		}
	}
}
