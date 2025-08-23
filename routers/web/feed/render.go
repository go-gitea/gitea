// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"code.gitea.io/gitea/services/context"
)

// RenderBranchFeed render format for branch or file
func RenderBranchFeedRSS(ctx *context.Context) {
	if ctx.Repo.TreePath == "" {
		ShowBranchFeed(ctx, ctx.Repo.Repository, "rss")
	} else {
		ShowFileFeed(ctx, ctx.Repo.Repository, "rss")
	}
}

func RenderBranchFeedAtom(ctx *context.Context) {
	if ctx.Repo.TreePath == "" {
		ShowBranchFeed(ctx, ctx.Repo.Repository, "atom")
	} else {
		ShowFileFeed(ctx, ctx.Repo.Repository, "atom")
	}
}
