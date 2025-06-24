// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/renderhelper"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/services/context"
	feed_service "code.gitea.io/gitea/services/feed"

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
	isOrganisation := ctx.ContextUser.IsOrganization()
	if ctx.IsSigned && isOrganisation && !includePrivate {
		// When feed is requested by a member of the organization,
		// include the private repo's the member has access to.
		isOrgMember, err := organization.IsOrganizationMember(ctx, ctx.ContextUser.ID, ctx.Doer.ID)
		if err != nil {
			ctx.ServerError("IsOrganizationMember", err)
			return
		}
		includePrivate = isOrgMember
	}

	actions, _, err := feed_service.GetFeeds(ctx, activities_model.GetFeedsOptions{
		RequestedUser:   ctx.ContextUser,
		Actor:           ctx.Doer,
		IncludePrivate:  includePrivate,
		OnlyPerformedBy: !isOrganisation,
		IncludeDeleted:  false,
		Date:            ctx.FormString("date"),
	})
	if err != nil {
		ctx.ServerError("GetFeeds", err)
		return
	}

	rctx := renderhelper.NewRenderContextSimpleDocument(ctx, ctx.ContextUser.HTMLURL())
	ctxUserDescription, err := markdown.RenderString(rctx,
		ctx.ContextUser.Description)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}

	feed := &feeds.Feed{
		Title:       ctx.Locale.TrString("home.feed_of", ctx.ContextUser.DisplayName()),
		Link:        &feeds.Link{Href: ctx.ContextUser.HTMLURL()},
		Description: string(ctxUserDescription),
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
