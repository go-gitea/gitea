package feed

import (
	"time"

	activities_model "gitea.dev/models/activities"
	group_model "gitea.dev/models/group"
	"gitea.dev/modules/httplib"
	"gitea.dev/services/context"
	feed_service "gitea.dev/services/feed"
	"github.com/gorilla/feeds"
)

// ShowGroupFeed shows user activity on the repo as RSS / Atom feed
func ShowGroupFeed(ctx *context.Context, group *group_model.Group, formatType string) {
	actions, _, err := feed_service.GetFeeds(ctx, activities_model.GetFeedsOptions{
		RequestedGroup: group,
		Actor:          ctx.Doer,
		IncludePrivate: true,
		Date:           ctx.FormString("date"),
	})
	if err != nil {
		ctx.ServerError("GetFeeds", err)
		return
	}
	htmlURL := httplib.MakeAbsoluteURL(ctx, group.GroupLink())

	feed := &feeds.Feed{
		Title:       ctx.Locale.TrString("home.feed_of", group.Name),
		Link:        &feeds.Link{Href: htmlURL},
		Description: group.Description,
		Created:     time.Now(),
	}

	feed.Items, err = feedActionsToFeedItems(ctx, actions)
	if err != nil {
		ctx.ServerError("convert feed", err)
		return
	}

	writeFeed(ctx, feed, formatType)
}
