// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"context"

	"gitea.dev/models/user"
	"gitea.dev/modules/markup"
	gitea_context "gitea.dev/services/context"
)

func FormalRenderHelperFuncs() *markup.RenderHelperFuncs {
	return &markup.RenderHelperFuncs{
		RenderRepoFileCodePreview: renderRepoFileCodePreview,
		RenderRepoIssueIconTitle:  renderRepoIssueIconTitle,
		IsUsernameMentionable: func(ctx context.Context, username string) bool {
			mentionedUser, err := user.GetUserByName(ctx, username)
			if err != nil {
				return false
			}

			giteaCtx := gitea_context.GetWebContext(ctx)
			if giteaCtx == nil {
				// when using general context, use user's visibility to check
				return mentionedUser.Visibility.IsPublic()
			}

			// when using gitea context (web context), use user's visibility and user's permission to check
			return user.IsUserVisibleToViewer(giteaCtx, mentionedUser, giteaCtx.Doer)
		},
	}
}
