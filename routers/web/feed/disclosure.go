// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"fmt"
	"time"

	"gitea.dev/models/db"
	disclosure_model "gitea.dev/models/disclosure"
	"gitea.dev/models/renderhelper"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/services/context"

	"github.com/gorilla/feeds"
)

// disclosuresFeedMaxItems caps how many disclosures are served in one feed
const disclosuresFeedMaxItems = 50

// ShowDisclosureFeed serves a repo's security disclosures as RSS / Atom feed.
// Drafts are never included (IncludeDrafts is false); the route already requires security-unit read access.
func ShowDisclosureFeed(ctx *context.Context, repo *repo_model.Repository, formatType string) {
	if !checkRepoFeedTokenScope(ctx) {
		return
	}
	disclosures, err := disclosure_model.ListDisclosures(ctx, disclosure_model.ListDisclosuresOptions{
		RepoID:      repo.ID,
		ListOptions: db.ListOptions{Page: 1, PageSize: disclosuresFeedMaxItems},
		OrderBy:     "newest",
	})
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	feed := &feeds.Feed{
		Title:       ctx.Locale.TrString("repo.security.feed_title", repo.FullName()),
		Link:        &feeds.Link{Href: repo.HTMLURL() + "/security/disclosures"},
		Description: repo.Description,
		Created:     time.Now(),
	}

	items := make([]*feeds.Item, 0, len(disclosures))
	for _, d := range disclosures {
		link := &feeds.Link{Href: fmt.Sprintf("%s/security/disclosures/%d", repo.HTMLURL(), d.ID)}

		desc := ctx.Locale.TrString("repo.security.severity_"+string(d.Severity)) + ": " + d.Summary
		if d.Status == disclosure_model.StatusWithdrawn {
			desc = ctx.Locale.TrString("repo.security.status_withdrawn") + " — " + desc
		}

		rctx := renderhelper.NewRenderContextRepoComment(ctx, repo).WithUseAbsoluteLink(true)
		content, err := markdown.RenderString(rctx, d.Content)
		if err != nil {
			ctx.ServerErrorAuto(err)
			return
		}

		author := d.Publisher
		if author == nil {
			author = d.Author
		}
		created := d.CreatedUnix.AsTime()
		if d.PublishedAt > 0 {
			created = d.PublishedAt.AsTime()
		}

		item := &feeds.Item{
			Title:       d.Identifier() + ": " + d.Title,
			Link:        link,
			Description: desc,
			Id:          fmt.Sprintf("%d: %s", d.ID, link.Href),
			Created:     created,
			Updated:     d.UpdatedUnix.AsTime(),
			Content:     string(content),
		}
		if author != nil {
			item.Author = &feeds.Author{Name: author.GetDisplayName(), Email: author.GetEmail()}
		}
		items = append(items, item)
	}
	feed.Items = items

	writeFeed(ctx, feed, formatType)
}
