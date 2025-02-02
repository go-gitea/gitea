// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/services/context"
	feed_service "code.gitea.io/gitea/services/feed"

	"github.com/gorilla/feeds"
)

// ShowRepoFeed shows user activity on the repo as RSS / Atom feed
func ShowRepoFeed(ctx *context.Context, repo *repo_model.Repository, formatType string) {
	actions, _, err := feed_service.GetFeeds(ctx, activities_model.GetFeedsOptions{
		RequestedRepo:  repo,
		Actor:          ctx.Doer,
		IncludePrivate: true,
		Date:           ctx.FormString("date"),
	})
	if err != nil {
		ctx.ServerError("GetFeeds", err)
		return
	}

	feed := &feeds.Feed{
		Title:       ctx.Locale.TrString("home.feed_of", repo.FullName()),
		Link:        &feeds.Link{Href: repo.HTMLURL()},
		Description: repo.Description,
		Created:     time.Now(),
	}

	feed.Items, err = feedActionsToFeedItems(ctx, actions)
	if err != nil {
		ctx.ServerError("convert feed", err)
		return
	}

	writeFeed(ctx, feed, formatType)
}
