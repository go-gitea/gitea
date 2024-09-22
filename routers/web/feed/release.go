// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/services/context"

	"github.com/gorilla/feeds"
)

// shows tags and/or releases on the repo as RSS / Atom feed
func ShowReleaseFeed(ctx *context.Context, repo *repo_model.Repository, isReleasesOnly bool, formatType string) {
	releases, err := db.Find[repo_model.Release](ctx, repo_model.FindReleasesOptions{
		IncludeTags: !isReleasesOnly,
		RepoID:      ctx.Repo.Repository.ID,
	})
	if err != nil {
		ctx.ServerError("GetReleasesByRepoID", err)
		return
	}

	var title string
	var link *feeds.Link

	if isReleasesOnly {
		title = ctx.Locale.TrString("repo.release.releases_for", repo.FullName())
		link = &feeds.Link{Href: repo.HTMLURL() + "/release"}
	} else {
		title = ctx.Locale.TrString("repo.release.tags_for", repo.FullName())
		link = &feeds.Link{Href: repo.HTMLURL() + "/tags"}
	}

	feed := &feeds.Feed{
		Title:       title,
		Link:        link,
		Description: repo.Description,
		Created:     time.Now(),
	}

	feed.Items, err = releasesToFeedItems(ctx, releases)
	if err != nil {
		ctx.ServerError("releasesToFeedItems", err)
		return
	}

	writeFeed(ctx, feed, formatType)
}
