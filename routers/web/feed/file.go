// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/util"

	"github.com/gorilla/feeds"
)

// ShowFileFeed shows tags and/or releases on the repo as RSS / Atom feed
func ShowFileFeed(ctx *context.Context, repo *repo.Repository, formatType string) {
	fileName := ctx.Repo.TreePath
	if len(fileName) == 0 {
		return
	}
	commits, err := ctx.Repo.GitRepo.CommitsByFileAndRange(
		git.CommitsByFileAndRangeOptions{
			Revision: ctx.Repo.RefName,
			File:     fileName,
			Page:     1,
		})
	if err != nil {
		ctx.ServerError("ShowBranchFeed", err)
		return
	}

	title := fmt.Sprintf("Latest commits for file %s", ctx.Repo.TreePath)

	link := &feeds.Link{Href: repo.HTMLURL() + "/" + ctx.Repo.BranchNameSubURL() + "/" + util.PathEscapeSegments(ctx.Repo.TreePath)}

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
		})
	}

	writeFeed(ctx, feed, formatType)
}
