// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"code.gitea.io/gitea/services/context"
)

// RenderBranchFeed render format for branch or file
func RenderBranchFeed(ctx *context.Context, feedType string) {
	if ctx.Repo.TreePath == "" {
		ShowBranchFeed(ctx, ctx.Repo.Repository, feedType)
	} else {
		ShowFileFeed(ctx, ctx.Repo.Repository, feedType)
	}
}

func RenderBranchFeedRSS(ctx *context.Context) {
	RenderBranchFeed(ctx, "rss")
}

func RenderBranchFeedAtom(ctx *context.Context) {
	RenderBranchFeed(ctx, "atom")
}
