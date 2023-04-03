// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"fmt"

	model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

// RenderBranchFeedRss render rss format for branch feed
func RenderBranchFeedRss(ctx *context.Context) {
	render(ctx, "rss")
}

// RenderBranchFeedAtom render atom format for branch feed
func RenderBranchFeedAtom(ctx *context.Context) {
	render(ctx, "atom")
}

// RenderRepoFeed handles if an RSS feed should be rendered, injects variables into context if not.
//
// The logic for rendering as a rss / atom feed checks against:
// * existence of Accept header containing application/rss+xml or application/atom+xml
// * support for the {repo}.rss url
func RenderRepoFeed(ctx *context.Context) {
	if !setting.EnableFeed {
		return
	}
	isFeed, _, showFeedType := GetFeedType(ctx.Params(":reponame"), ctx.Req)
	if !isFeed {
		ctx.Data["EnableFeed"] = true
		ctx.Data["FeedURL"] = ctx.Repo.Repository.HTMLURL()
		return
	}
	render(ctx, showFeedType)
}

// render
func render(ctx *context.Context, showFeedType string) {
	var renderer func(ctx *context.Context, repo *model.Repository, formatType string)
	switch {
	case ctx.Link == fmt.Sprintf("%s.%s", ctx.Repo.RepoLink, showFeedType):
		renderer = ShowRepoFeed
	case ctx.Repo.TreePath == "":
		renderer = ShowBranchFeed
	case ctx.Repo.TreePath != "":
		renderer = ShowFileFeed
	}
	renderer(ctx, ctx.Repo.Repository, showFeedType)
}
