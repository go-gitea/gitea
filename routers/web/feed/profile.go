// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"

	"github.com/gorilla/feeds"
)

// ShowUserFeedRSS show user activity as RSS feed
func ShowUserFeedRSS(ctx *context.Context) {
	showUserFeed(ctx, "rss")
}

// ShowUserFeedAtom show user activity as Atom feed
func ShowUserFeedAtom(ctx *context.Context) {
	showUserFeed(ctx, "atom")
}

// showUserFeed show user activity as RSS / Atom feed
func showUserFeed(ctx *context.Context, formatType string) {
	includePrivate := ctx.IsSigned && (ctx.Doer.IsAdmin || ctx.Doer.ID == ctx.ContextUser.ID)

	actions, _, err := activities_model.GetFeeds(ctx, activities_model.GetFeedsOptions{
		RequestedUser:   ctx.ContextUser,
		Actor:           ctx.Doer,
		IncludePrivate:  includePrivate,
		OnlyPerformedBy: !ctx.ContextUser.IsOrganization(),
		IncludeDeleted:  false,
		Date:            ctx.FormString("date"),
	})
	if err != nil {
		ctx.ServerError("GetFeeds", err)
		return
	}

	ctxUserDescription, err := markdown.RenderString(&markup.RenderContext{
		Ctx:       ctx,
		URLPrefix: ctx.ContextUser.HTMLURL(),
		Metas: map[string]string{
			"user": ctx.ContextUser.GetDisplayName(),
		},
	}, ctx.ContextUser.Description)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}

	feed := &feeds.Feed{
		Title:       ctx.Tr("home.feed_of", ctx.ContextUser.DisplayName()),
		Link:        &feeds.Link{Href: ctx.ContextUser.HTMLURL()},
		Description: ctxUserDescription,
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
