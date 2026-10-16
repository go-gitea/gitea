// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	auth_model "gitea.dev/models/auth"
	"gitea.dev/services/context"
)

// checkRepoFeedTokenScope ensures an API token has repository read scope before a
// feed serves private repository content, mirroring checkDownloadTokenScope for
// downloads. Returns false (and writes the response) when the token is denied.
func checkRepoFeedTokenScope(ctx *context.Context) bool {
	context.CheckRepoScopedToken(ctx, ctx.Repo.Repository, auth_model.Read)
	return !ctx.Written()
}

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
