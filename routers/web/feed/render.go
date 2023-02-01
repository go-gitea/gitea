// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"fmt"

	model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

func RenderFeed(ctx *context.Context) bool {
	if !setting.EnableFeed {
		return false
	}
	isFeed, _, showFeedType := GetFeedType(ctx.Link, ctx.Req)
	if !isFeed {
		ctx.Data["EnableFeed"] = true
		ctx.Data["FeedURL"] = ctx.Repo.Repository.HTMLURL()
		return false
	}

	var renderer func(ctx *context.Context, repo *model.Repository, formatType string)
	switch {
	case ctx.Link == fmt.Sprintf("%s.%s", ctx.Repo.RepoLink, showFeedType):
		renderer = ShowRepoFeed
	case ctx.Repo.TreePath == "":
		renderer = ShowBranchFeed
	case ctx.Repo.TreePath != "":
		renderer = ShowFileFeed
	default:
		return false
	}
	renderer(ctx, ctx.Repo.Repository, showFeedType)
	return true
}
