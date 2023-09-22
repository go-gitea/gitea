// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"time"

	repo_model "code.gitea.io/gitea/internal/models/repo"
	"code.gitea.io/gitea/internal/modules/context"

	"github.com/gorilla/feeds"
)

// shows tags and/or releases on the repo as RSS / Atom feed
func ShowReleaseFeed(ctx *context.Context, repo *repo_model.Repository, isReleasesOnly bool, formatType string) {
	releases, err := repo_model.GetReleasesByRepoID(ctx, ctx.Repo.Repository.ID, repo_model.FindReleasesOptions{
		IncludeTags: !isReleasesOnly,
	})
	if err != nil {
		ctx.ServerError("GetReleasesByRepoID", err)
		return
	}

	var title string
	var link *feeds.Link

	if isReleasesOnly {
		title = ctx.Tr("repo.release.releases_for", repo.FullName())
		link = &feeds.Link{Href: repo.HTMLURL() + "/release"}
	} else {
		title = ctx.Tr("repo.release.tags_for", repo.FullName())
		link = &feeds.Link{Href: repo.HTMLURL() + "/tags"}
	}

	feed := &feeds.Feed{
		Title:       title,
		Link:        link,
		Description: repo.Description,
		Created:     time.Now(),
	}

	feed.Items, err = releasesToFeedItems(ctx, releases, isReleasesOnly)
	if err != nil {
		ctx.ServerError("releasesToFeedItems", err)
		return
	}

	writeFeed(ctx, feed, formatType)
}
