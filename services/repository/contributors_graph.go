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
)

// ErrAwaitGeneration is returned while the contributor stats of a repository are generated for the first time
var ErrAwaitGeneration = errors.New("contributor stats are being generated")

type WeekData struct {
	Week      int64 `json:"week"`      // Starting day of the week as Unix timestamp
	Additions int   `json:"additions"` // Number of additions in that week
	Deletions int   `json:"deletions"` // Number of deletions in that week
	Commits   int   `json:"commits"`   // Number of commits in that week
}

// ContributorData represents statistical git commit count data
type ContributorData struct {
	Name         string              `json:"name"`  // Display name of the contributor
	Login        string              `json:"login"` // Login name of the contributor in case it exists
	AvatarLink   string              `json:"avatar_link"`
	HomeLink     string              `json:"home_link"`
	TotalCommits int64               `json:"total_commits"`
	Weeks        map[int64]*WeekData `json:"weeks"`
}

// ensureContributorStats queues the generation of the contributor stats of a repository if they were never generated
func ensureContributorStats(ctx context.Context, repo *repo_model.Repository) error {
	if repo.IsEmpty {
		return nil
	}
	status, err := repo_model.GetIndexerStatus(ctx, repo, repo_model.RepoIndexerTypeContributors)
	if err != nil {
		return err
	}
	if status.CommitSha != "" {
		return nil
	}
	if err := AddRepoToContributorsQueue(repo.ID); err != nil {
		return err
	}
	return ErrAwaitGeneration
}

// FindRepoContributors returns the contributors of a repository with their users loaded, most commits first
func FindRepoContributors(ctx context.Context, repo *repo_model.Repository, opts repo_model.FindRepoContributorsOptions) (repo_model.RepoContributorList, int64, error) {
	if err := ensureContributorStats(ctx, repo); err != nil {
		return nil, 0, err
	}
	opts.RepoID = repo.ID
	contributors, total, err := db.FindAndCount[repo_model.RepoContributor](ctx, opts)
	if err != nil {
		return nil, 0, err
	}
	return contributors, total, repo_model.RepoContributorList(contributors).LoadUsers(ctx)
}

// GetContributorStats returns the weekly stats of the default branch contributors keyed by email, and their sum keyed by "total"
func GetContributorStats(ctx context.Context, repo *repo_model.Repository) (map[string]*ContributorData, error) {
	contributors, _, err := FindRepoContributors(ctx, repo, repo_model.FindRepoContributorsOptions{ListOptions: db.ListOptionsAll})
	if err != nil {
		return nil, err
	}
	weeks, err := repo_model.GetRepoContributorWeeks(ctx, repo.ID)
	if err != nil {
		return nil, err
	}
	total, err := getWeeklyTotals(ctx, repo)
	if err != nil {
		return nil, err
	}

	ghostAvatarLink := user_model.NewGhostUser().AvatarLinkWithSize(ctx, 0)
	stats := make(map[string]*ContributorData, len(contributors)+1)
	byKey := make(map[contributorKey]*ContributorData, len(contributors))
	for _, c := range contributors {
		data := &ContributorData{TotalCommits: c.Commits, Weeks: make(map[int64]*WeekData)}
		key := c.Email
		if c.User != nil {
			key = cmp.Or(c.User.GetEmail(), fmt.Sprintf("ghost-%d", c.UserID))
			data.Name, data.AvatarLink = c.User.DisplayName(), c.User.AvatarLinkWithSize(ctx, 0)
			if c.User.ID > 0 {
				data.Login, data.HomeLink = c.User.LowerName, c.User.HomeLink()
			}
		} else {
			data.Name, data.AvatarLink = c.Name, cmp.Or(avatars.GenerateEmailAvatarFastLink(ctx, c.Email, 0), ghostAvatarLink)
		}
		stats[key] = data
		byKey[contributorKey{c.UserID, c.Email}] = data
	}
	for _, w := range weeks {
		if data := byKey[contributorKey{w.UserID, w.Email}]; data != nil {
			data.Weeks[w.Week] = &WeekData{Week: w.Week, Additions: int(w.Additions), Deletions: int(w.Deletions), Commits: int(w.Commits)}
		}
	}

	totalData := &ContributorData{Name: "Total", Weeks: total}
	for _, w := range total {
		totalData.TotalCommits += int64(w.Commits)
	}
	stats["total"] = totalData
	return stats, nil
}

// GetWeeklyTotals returns the stats of all default branch contributors summed up per week
func GetWeeklyTotals(ctx context.Context, repo *repo_model.Repository) (map[int64]*WeekData, error) {
	if err := ensureContributorStats(ctx, repo); err != nil {
		return nil, err
	}
	return getWeeklyTotals(ctx, repo)
}

func getWeeklyTotals(ctx context.Context, repo *repo_model.Repository) (map[int64]*WeekData, error) {
	weeks, err := repo_model.GetRepoWeeklyTotals(ctx, repo.ID)
	if err != nil {
		return nil, err
	}
	totals := make(map[int64]*WeekData, len(weeks))
	for _, w := range weeks {
		totals[w.Week] = &WeekData{Week: w.Week, Additions: int(w.Additions), Deletions: int(w.Deletions), Commits: int(w.Commits)}
	}
	return totals, nil
}
