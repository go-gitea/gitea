// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"

	admin_model "code.gitea.io/gitea/models/admin"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
)

// SetEditorconfigIfExists set editor config as render variable
func SetEditorconfigIfExists(ctx *context.Context) {
	if ctx.Repo.Repository.IsEmpty {
		ctx.Data["Editorconfig"] = nil
		return
	}

	ec, err := ctx.Repo.GetEditorconfig()

	if err != nil && !git.IsErrNotExist(err) {
		description := fmt.Sprintf("Error while getting .editorconfig file: %v", err)
		if err := admin_model.CreateRepositoryNotice(description); err != nil {
			ctx.ServerError("ErrCreatingReporitoryNotice", err)
		}
		return
	}

	ctx.Data["Editorconfig"] = ec
}

// SetDiffViewStyle set diff style as render variable
func SetDiffViewStyle(ctx *context.Context) {
	queryStyle := ctx.FormString("style")

	if !ctx.IsSigned {
		ctx.Data["IsSplitStyle"] = queryStyle == "split"
		return
	}

	var (
		userStyle = ctx.Doer.DiffViewStyle
		style     string
	)

	if queryStyle == "unified" || queryStyle == "split" {
		style = queryStyle
	} else if userStyle == "unified" || userStyle == "split" {
		style = userStyle
	} else {
		style = "unified"
	}

	ctx.Data["IsSplitStyle"] = style == "split"
	if err := user_model.UpdateUserDiffViewStyle(ctx.Doer, style); err != nil {
		ctx.ServerError("ErrUpdateDiffViewStyle", err)
	}
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
		userWhitespaceBehavior, err := user_model.GetUserSetting(ctx.Doer.ID, user_model.SettingsKeyDiffWhitespaceBehavior, defaultWhitespaceBehavior)
		if err == nil {
			if whitespaceBehavior == "" {
				whitespaceBehavior = userWhitespaceBehavior
			} else if whitespaceBehavior != userWhitespaceBehavior {
				_ = user_model.SetUserSetting(ctx.Doer.ID, user_model.SettingsKeyDiffWhitespaceBehavior, whitespaceBehavior)
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
