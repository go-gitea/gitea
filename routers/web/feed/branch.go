// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/services/context"

	"github.com/gorilla/feeds"
)

// ShowBranchFeed shows tags and/or releases on the repo as RSS / Atom feed
func ShowBranchFeed(ctx *context.Context, repo *repo.Repository, formatType string) {
	commits, err := ctx.Repo.Commit.CommitsByRange(0, 10, "")
	if err != nil {
		ctx.ServerError("ShowBranchFeed", err)
		return
	}

	title := fmt.Sprintf("Latest commits for branch %s", ctx.Repo.BranchName)
	link := &feeds.Link{Href: repo.HTMLURL() + "/" + ctx.Repo.RefTypeNameSubURL()}

	feed := &feeds.Feed{
		Title:       title,
		Link:        link,
		Description: repo.Description,
		Created:     time.Now(),
	}

	for _, commit := range commits {
		feed.Items = append(feed.Items, &feeds.Item{
			Id:    commit.ID.String(),
			Title: strings.TrimSpace(strings.Split(commit.Message(), "\n")[0]),
			Link:  &feeds.Link{Href: repo.HTMLURL() + "/commit/" + commit.ID.String()},
			Author: &feeds.Author{
				Name:  commit.Author.Name,
				Email: commit.Author.Email,
			},
			Description: commit.Message(),
			Content:     commit.Message(),
			Created:     commit.Committer.When,
		})
	}

	writeFeed(ctx, feed, formatType)
}
