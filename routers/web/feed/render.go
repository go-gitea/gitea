// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
)

// RenderBranchFeed render format for branch or file
func RenderBranchFeed(ctx *context.Context) {
	_, _, showFeedType := GetFeedType(ctx.Params(":reponame"), ctx.Req)
	var renderer func(ctx *context.Context, repo *model.Repository, formatType string)
	switch {
	case ctx.Repo.TreePath == "":
		renderer = ShowBranchFeed
	case ctx.Repo.TreePath != "":
		renderer = ShowFileFeed
	}
	renderer(ctx, ctx.Repo.Repository, showFeedType)
}
