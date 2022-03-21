// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package feed

import (
	"net/http"
	"time"

	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"

	"github.com/gorilla/feeds"
)

// ShowUserFeed show user activity as RSS / Atom feed
func ShowUserFeed(ctx *context.Context, ctxUser *user_model.User, formatType string) {
	actions, err := models.GetFeeds(ctx, models.GetFeedsOptions{
		RequestedUser:   ctxUser,
		Actor:           ctx.User,
		IncludePrivate:  false,
		OnlyPerformedBy: !ctxUser.IsOrganization(),
		IncludeDeleted:  false,
		Date:            ctx.FormString("date"),
	})
	if err != nil {
		ctx.ServerError("GetFeeds", err)
		return
	}

	feed := &feeds.Feed{
		Title:       ctx.Tr("home.feed_of", ctxUser.DisplayName()),
		Link:        &feeds.Link{Href: ctxUser.HTMLURL()},
		Description: ctxUser.Description,
		Created:     time.Now(),
	}

	feed.Items, err = feedActionsToFeedItems(ctx, actions)
	if err != nil {
		ctx.ServerError("convert feed", err)
		return
	}

	writeFeed(ctx, feed, formatType)
}

// writeFeed write a feeds.Feed as atom or rss to ctx.Resp
func writeFeed(ctx *context.Context, feed *feeds.Feed, formatType string) {
	ctx.Resp.WriteHeader(http.StatusOK)
	if formatType == "atom" {
		ctx.Resp.Header().Set("Content-Type", "application/atom+xml;charset=utf-8")
		if err := feed.WriteAtom(ctx.Resp); err != nil {
			ctx.ServerError("Render Atom failed", err)
		}
	} else {
		ctx.Resp.Header().Set("Content-Type", "application/rss+xml;charset=utf-8")
		if err := feed.WriteRss(ctx.Resp); err != nil {
			ctx.ServerError("Render RSS failed", err)
		}
	}
}
