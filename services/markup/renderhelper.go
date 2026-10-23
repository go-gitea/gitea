// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"context"

	"codenest.dev/models/user"
	"codenest.dev/modules/markup"
	codenest_context "codenest.dev/services/context"
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

			codenestCtx := codenest_context.GetWebContext(ctx)
			if codenestCtx == nil {
				// when using general context, use user's visibility to check
				return mentionedUser.Visibility.IsPublic()
			}

			// when using codenest context (web context), use user's visibility and user's permission to check
			return user.IsUserVisibleToViewer(codenestCtx, mentionedUser, codenestCtx.Doer)
		},
	}
}
