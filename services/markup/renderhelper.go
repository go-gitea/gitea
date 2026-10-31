// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"context"

	"gitea.dev/models/user"
	"gitea.dev/modules/markup"
	"gitea.dev/modules/setting"
	gitea_context "gitea.dev/services/context"
)

func FormalRenderHelperFuncs() *markup.RenderHelperFuncs {
	return &markup.RenderHelperFuncs{
		RenderRepoFileCodePreview: renderRepoFileCodePreview,
		RenderRepoIssueIconTitle:  renderRepoIssueIconTitle,
		ResolveUserMention: func(ctx context.Context, username string) (bool, string) {
			mentionedUser, err := user.GetUserByName(ctx, username)
			if err != nil {
				return false, ""
			}

			giteaCtx := gitea_context.GetWebContext(ctx)
			if giteaCtx == nil {
				// when using general context, use user's visibility to check
				if !mentionedUser.Visibility.IsPublic() {
					return false, ""
				}
			} else if !user.IsUserVisibleToViewer(giteaCtx, mentionedUser, giteaCtx.Doer) {
				// when using gitea context (web context), use user's visibility and user's permission to check
				return false, ""
			}
			return true, mentionedUser.AvatarLinkWithSize(ctx, 20*setting.Avatar.RenderedSizeFactor)
		},
	}
}
