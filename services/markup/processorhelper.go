// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"context"

	"code.gitea.io/gitea/models/user"
	module_context "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
)

func ProcessorHelper() *markup.ProcessorHelper {
	return &markup.ProcessorHelper{
		IsUsernameMentionable: func(ctx context.Context, username string) bool {
			mentionedUser, err := user.GetUserByName(ctx, username)
			if err != nil {
				return false
			}

			moduleCtx, ok := ctx.(*module_context.Context)
			if !ok {
				log.Error("couldn't cast context, assuming user should be visible based on their visibility")
				return mentionedUser.Visibility.IsPublic()
			}

			return user.IsUserVisibleToViewer(moduleCtx, mentionedUser, moduleCtx.Doer)
		},
	}
}
