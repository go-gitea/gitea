// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"context"

	"code.gitea.io/gitea/models/user"
	gitea_context "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markup"
)

func ProcessorHelper() *markup.ProcessorHelper {
	return &markup.ProcessorHelper{
		ElementDir: "auto", // set dir="auto" for necessary (eg: <p>, <h?>, etc) tags
		IsUsernameMentionable: func(ctx context.Context, username string) bool {
			mentionedUser, err := user.GetUserByName(ctx, username)
			if err != nil {
				return false
			}

			giteaCtx, ok := ctx.(*gitea_context.Context)
			if !ok {
				// when using general context, use user's visibility to check
				return mentionedUser.Visibility.IsPublic()
			}

			// when using gitea context (web context), use user's visibility and user's permission to check
			return user.IsUserVisibleToViewer(giteaCtx, mentionedUser, giteaCtx.Doer)
		},
	}
}
