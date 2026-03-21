// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"strconv"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/gitdiff"
	user_service "code.gitea.io/gitea/services/user"
)

// SetEditorconfigIfExists set editor config as render variable
func SetEditorconfigIfExists(ctx *context.Context) {
	if ctx.Repo.Repository.IsEmpty {
		return
	}

	ec, _, err := ctx.Repo.GetEditorconfig()
	if err != nil {
		// it used to check `!git.IsErrNotExist(err)` and create a system notice, but it is quite annoying and useless
		// because network errors also happen frequently, so we just ignore it
		return
	}

	ctx.Data["Editorconfig"] = ec
}

func GetDiffViewStyle(ctx *context.Context) string {
	return util.Iif(ctx.Data["IsSplitStyle"] == true, gitdiff.DiffStyleSplit, gitdiff.DiffStyleUnified)
}

// SetDiffViewStyle set diff style as render variable
func SetDiffViewStyle(ctx *context.Context) {
	style := ctx.FormString("style")
	if ctx.IsSigned {
		style = util.IfZero(style, ctx.Doer.DiffViewStyle)
		style = util.Iif(style == gitdiff.DiffStyleSplit, gitdiff.DiffStyleSplit, gitdiff.DiffStyleUnified)
		if style != ctx.Doer.DiffViewStyle {
			err := user_service.UpdateUser(ctx, ctx.Doer, &user_service.UpdateOptions{DiffViewStyle: optional.Some(style)})
			if err != nil {
				log.Error("UpdateUser DiffViewStyle: %v", err)
			}
		}
	}
	ctx.Data["IsSplitStyle"] = style == "split"
}

// SetWhitespaceBehavior set whitespace behavior as render variable
func SetWhitespaceBehavior(ctx *context.Context) {
	const defaultWhitespaceBehavior = "show-all"
	whitespaceBehavior := ctx.FormString("whitespace")
	switch whitespaceBehavior {
	case "", "ignore-all", "ignore-eol", "ignore-change":
		break
	default:
		whitespaceBehavior = defaultWhitespaceBehavior
	}
	if ctx.IsSigned {
		userWhitespaceBehavior, err := user_model.GetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyDiffWhitespaceBehavior, defaultWhitespaceBehavior)
		if err == nil {
			if whitespaceBehavior == "" {
				whitespaceBehavior = userWhitespaceBehavior
			} else if whitespaceBehavior != userWhitespaceBehavior {
				_ = user_model.SetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyDiffWhitespaceBehavior, whitespaceBehavior)
			}
		} // else: we can ignore the error safely
	}

	// these behaviors are for gitdiff.GetWhitespaceFlag
	if whitespaceBehavior == "" {
		ctx.Data["WhitespaceBehavior"] = defaultWhitespaceBehavior
	} else {
		ctx.Data["WhitespaceBehavior"] = whitespaceBehavior
	}
}

// SetShowOutdatedComments set the show outdated comments option as context variable
func SetShowOutdatedComments(ctx *context.Context) {
	showOutdatedCommentsValue := ctx.FormString("show-outdated")
	if showOutdatedCommentsValue != "true" && showOutdatedCommentsValue != "false" {
		// invalid or no value for this form string -> use default or stored user setting
		showOutdatedCommentsValue = "true"
		if ctx.IsSigned {
			showOutdatedCommentsValue, _ = user_model.GetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyShowOutdatedComments, showOutdatedCommentsValue)
		}
	} else if ctx.IsSigned {
		// valid value -> update user setting if user is logged in
		_ = user_model.SetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyShowOutdatedComments, showOutdatedCommentsValue)
	}
	ctx.Data["ShowOutdatedComments"], _ = strconv.ParseBool(showOutdatedCommentsValue)
}
