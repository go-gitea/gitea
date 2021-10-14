// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package feed

import (
	"net/http"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"

	"github.com/gorilla/feeds"
)

// RetrieveFeeds loads feeds for the specified user
func RetrieveFeeds(ctx *context.Context, options models.GetFeedsOptions) []*models.Action {
	actions, err := models.GetFeeds(options)
	if err != nil {
		ctx.ServerError("GetFeeds", err)
		return nil
	}

	userCache := map[int64]*models.User{options.RequestedUser.ID: options.RequestedUser}
	if ctx.User != nil {
		userCache[ctx.User.ID] = ctx.User
	}
	for _, act := range actions {
		if act.ActUser != nil {
			userCache[act.ActUserID] = act.ActUser
		}
	}

	for _, act := range actions {
		repoOwner, ok := userCache[act.Repo.OwnerID]
		if !ok {
			repoOwner, err = models.GetUserByID(act.Repo.OwnerID)
			if err != nil {
				if models.IsErrUserNotExist(err) {
					continue
				}
				ctx.ServerError("GetUserByID", err)
				return nil
			}
			userCache[repoOwner.ID] = repoOwner
		}
		act.Repo.Owner = repoOwner
	}
	return actions
}

// ShowUserFeed show user activity as RSS / Atom feed
func ShowUserFeed(ctx *context.Context, ctxUser *models.User, formatType string) {
	actions := RetrieveFeeds(ctx, models.GetFeedsOptions{
		RequestedUser:   ctxUser,
		Actor:           ctx.User,
		IncludePrivate:  false,
		OnlyPerformedBy: true,
		IncludeDeleted:  false,
		Date:            ctx.FormString("date"),
	})
	if ctx.Written() {
		return
	}

	feed := &feeds.Feed{
		Title:       ctx.Tr("home.feed_of", ctxUser.DisplayName()),
		Link:        &feeds.Link{Href: ctxUser.HTMLURL()},
		Description: ctxUser.Description,
		Created:     time.Now(),
	}

	var err error
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
