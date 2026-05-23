// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"errors"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
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

func blockedUsersPost(ctx *context.Context, form *forms.BlockUserForm, blocker *user_model.User) error {
	blockee, err := user_model.GetUserByName(ctx, form.Blockee)
	if err != nil {
		return err
	}

	switch form.Action {
	case "block":
		err = user_service.BlockUser(ctx, ctx.Doer, blocker, blockee, form.Note)
		if errors.Is(err, util.ErrInvalidArgument) {
			return util.ErrorWrapTranslatable(err, "user.block.block.failure", err.Error())
		}
		return err
	case "unblock":
		err = user_service.UnblockUser(ctx, ctx.Doer, blocker, blockee)
		if errors.Is(err, util.ErrInvalidArgument) {
			return util.ErrorWrapTranslatable(err, "user.block.unblock.failure", err.Error())
		}
		return err
	case "note":
		block, err := user_model.GetBlocking(ctx, blocker.ID, blockee.ID)
		if err != nil {
			return err
		}
		return user_model.UpdateBlockingNote(ctx, block.ID, form.Note)
	}
	setting.PanicInDevOrTesting("Unknown action: %q", form.Action)
	return errors.New("unknown action")
}

func BlockedUsersPost(ctx *context.Context, blocker *user_model.User, redirect string) {
	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	form := web.GetForm(ctx).(*forms.BlockUserForm)
	err := blockedUsersPost(ctx, form, blocker)
	if err == nil {
		ctx.JSONRedirect(redirect)
	} else if errTr := util.ErrorAsTranslatable(err); errTr != nil {
		ctx.JSONError(errTr.Translate(ctx.Locale))
	} else if errors.Is(err, util.ErrNotExist) {
		ctx.JSONError(ctx.Locale.Tr("error.not_found"))
	} else {
		ctx.ServerError("BlockedUsersPost", err)
	}
}
