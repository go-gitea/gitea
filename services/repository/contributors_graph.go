// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"cmp"
	"context"
	"errors"
	"fmt"

	"gitea.dev/models/avatars"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/cache"
	"gitea.dev/modules/container"
	"gitea.dev/modules/util"
)

var ErrAwaitGeneration = errors.New("contributor stats are being generated")

type WeekData struct {
	Week      int64 `json:"week"`      // Starting day of the week as Unix timestamp
	Additions int   `json:"additions"` // Number of additions in that week
	Deletions int   `json:"deletions"` // Number of deletions in that week
	Commits   int   `json:"commits"`   // Number of commits in that week
}

// ContributorData represents statistical git commit count data
type ContributorData struct {
	Name       string              `json:"name"` // Display name of the contributor
	AvatarLink string              `json:"avatar_link"`
	HomeLink   string              `json:"home_link"`
	Weeks      map[int64]*WeekData `json:"weeks"`
}

func EnsureContributorStats(ctx context.Context, repo *repo_model.Repository) error {
	headID, countedID, claimed, err := getContributorStatsCommits(ctx, repo)
	if err != nil || (headID == countedID && !claimed) {
		return err
	}
	if countErr, failed := cache.GetCache().Get(contributorsErrorCacheKey(repo.ID)); failed {
		return util.Iif(countedID == "", errors.New(countErr), nil)
	}
	if err := AddRepoToContributorsQueue(repo.ID); err != nil {
		return err
	}
	return util.Iif(countedID == "", ErrAwaitGeneration, nil)
}

func FindRepoContributors(ctx context.Context, repo *repo_model.Repository, opts repo_model.FindRepoContributorsOptions) ([]*repo_model.RepoContributor, int64, error) {
	opts.RepoID = repo.ID
	contributors, total, err := db.FindAndCount[repo_model.RepoContributor](ctx, opts)
	if err != nil {
		return nil, 0, err
	}
	users, err := user_model.GetUsersMapByIDs(ctx, container.FilterSlice(contributors, func(contributor *repo_model.RepoContributor) (int64, bool) {
		return contributor.UserID, contributor.UserID > 0
	}))
	if err != nil {
		return nil, 0, err
	}
	for _, contributor := range contributors {
		contributor.User = user_model.GetPossibleUserFromMap(contributor.UserID, users)
	}
	return contributors, total, nil
}

func GetContributorStats(ctx context.Context, repo *repo_model.Repository) (map[string]*ContributorData, error) {
	if err := EnsureContributorStats(ctx, repo); err != nil {
		return nil, err
	}
	contributors, _, err := FindRepoContributors(ctx, repo, repo_model.FindRepoContributorsOptions{ListOptions: db.ListOptionsAll})
	if err != nil {
		return nil, err
	}
	weeks, err := repo_model.GetRepoContributorWeeks(ctx, repo.ID)
	if err != nil {
		return nil, err
	}

	ghostAvatarLink := user_model.NewGhostUser().AvatarLinkWithSize(ctx, 0)
	stats := make(map[string]*ContributorData, len(contributors)+1)
	byKey := make(map[contributorKey]*ContributorData, len(contributors))
	for _, c := range contributors {
		key := c.Email
		if c.User != nil {
			key = cmp.Or(c.User.GetEmail(), fmt.Sprintf("ghost-%d", c.UserID))
		}
		// an email counted before it was added to a user also has a row of its own
		data := stats[key]
		if data == nil {
			data = &ContributorData{Weeks: make(map[int64]*WeekData)}
			stats[key] = data
		}
		if c.User != nil {
			data.Name, data.AvatarLink = c.User.DisplayName(), c.User.AvatarLinkWithSize(ctx, 0)
			if c.User.ID > 0 {
				data.HomeLink = c.User.HomeLink()
			}
		} else if data.Name == "" {
			data.Name, data.AvatarLink = c.Name, cmp.Or(avatars.GenerateEmailAvatarFastLink(ctx, c.Email, 0), ghostAvatarLink)
		}
		byKey[contributorKey{c.UserID, c.Email}] = data
	}
	total := &ContributorData{Name: "Total", Weeks: make(map[int64]*WeekData)}
	for _, w := range weeks {
		total.addWeek(w)
		if data := byKey[contributorKey{w.UserID, w.Email}]; data != nil {
			data.addWeek(w)
		}
	}
	stats["total"] = total
	return stats, nil
}

func (data *ContributorData) addWeek(w *repo_model.RepoContributorWeek) {
	week := data.Weeks[w.Week]
	if week == nil {
		week = &WeekData{Week: w.Week}
		data.Weeks[w.Week] = week
	}
	week.Additions += int(w.Additions)
	week.Deletions += int(w.Deletions)
	week.Commits += int(w.Commits)
}

func GetWeeklyTotals(ctx context.Context, repo *repo_model.Repository) (map[int64]*WeekData, error) {
	if err := EnsureContributorStats(ctx, repo); err != nil {
		return nil, err
	}
	weeks, err := repo_model.GetRepoWeeklyTotals(ctx, repo.ID)
	if err != nil {
		return nil, err
	}
	total := &ContributorData{Weeks: make(map[int64]*WeekData, len(weeks))}
	for _, w := range weeks {
		total.addWeek(w)
	}
	return total.Weeks, nil
}
