// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"gitea.dev/models/avatars"
	repo_model "gitea.dev/models/repo"
	contribution_model "gitea.dev/models/repo/contribution"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/gitrepo"
	api "gitea.dev/modules/structs"
)

var ErrAwaitGeneration = errors.New("contributor stats generation in progress")

// ContributorData represents statistical git commit count data
type ContributorData struct {
	Name         string                                 `json:"name"`  // Display name of the contributor
	Login        string                                 `json:"login"` // Login name of the contributor in case it exists
	AvatarLink   string                                 `json:"avatar_link"`
	HomeLink     string                                 `json:"home_link"`
	TotalCommits int64                                  `json:"total_commits"`
	Weeks        map[int64]*contribution_model.WeekData `json:"weeks"`
}

// ExtendedCommitStats contains information for commit stats with author data
type ExtendedCommitStats struct {
	Author *api.CommitUser  `json:"author"`
	Stats  *api.CommitStats `json:"stats"`
}

// GetContributorStats returns contributors stats for git commits for given revision or default branch.
func GetContributorStats(ctx context.Context, repo *repo_model.Repository, limit int, start, end *time.Time) (map[string]*ContributorData, error) {
	if repo.IsEmpty {
		return map[string]*ContributorData{"total": {
			Name: "Total",
		}}, nil
	}

	var startDay, endDay *contribution_model.ContributorDayStart
	if start != nil {
		value := contribution_model.NewContributorDayStart(start.UTC())
		startDay = &value
	}
	if end != nil {
		value := contribution_model.NewContributorDayStart(end.UTC())
		endDay = &value
	}
	stats, err := contribution_model.GetRepoContributorDailyStatsRange(ctx, repo.ID, startDay, endDay)
	if err != nil {
		return nil, err
	}
	hasStats := len(stats) > 0
	if !hasStats && (startDay != nil || endDay != nil) {
		hasStats, err = contribution_model.HasRepoContributorDailyStats(ctx, repo.ID)
		if err != nil {
			return nil, err
		}
	}

	if !hasStats {
		if err := RequestContributorStatsRebuild(ctx, repo.ID); err != nil && !errors.Is(err, ErrAwaitGeneration) {
			return nil, err
		}
		return nil, ErrAwaitGeneration
	}

	unknownUserAvatarLink := user_model.NewGhostUser().AvatarLinkWithSize(ctx, 0)
	contributorsCommitStats := make(map[string]*ContributorData)
	contributorsCommitStats["total"] = &ContributorData{
		Name: "Total",
	}

	userIDs := make(map[int64]struct{})
	for _, stat := range stats {
		if stat.UserID > 0 {
			userIDs[stat.UserID] = struct{}{}
		}
	}
	ids := make([]int64, 0, len(userIDs))
	for id := range userIDs {
		ids = append(ids, id)
	}
	users, err := user_model.GetUsersByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	userMap := make(map[int64]*user_model.User, len(users))
	for _, user := range users {
		userMap[user.ID] = user
	}

	for _, stat := range stats {
		if stat.Email == "" && stat.UserID == 0 {
			continue
		}
		user := userMap[stat.UserID]
		email := strings.ToLower(stat.Email)
		if email == "" && user != nil {
			email = strings.ToLower(user.GetEmail())
		}
		if email == "" {
			continue
		}

		if _, ok := contributorsCommitStats[email]; !ok {
			if user == nil {
				name := stat.AuthorName
				if name == "" {
					name = email
				}
				avatarLink := avatars.GenerateEmailAvatarFastLink(ctx, email, 0)
				if avatarLink == "" {
					avatarLink = unknownUserAvatarLink
				}
				contributorsCommitStats[email] = &ContributorData{
					Name:       name,
					AvatarLink: avatarLink,
					Weeks:      make(map[int64]*contribution_model.WeekData),
				}
			} else {
				contributorsCommitStats[email] = &ContributorData{
					Name:       user.DisplayName(),
					Login:      user.LowerName,
					AvatarLink: user.AvatarLinkWithSize(ctx, 0),
					HomeLink:   user.HomeLink(),
					Weeks:      make(map[int64]*contribution_model.WeekData),
				}
			}
		}
		userStats := contributorsCommitStats[email]
		week := weekStartUnixMilliFromDayStart(stat.DayStart)

		if userStats.Weeks[week] == nil {
			userStats.Weeks[week] = &contribution_model.WeekData{
				Week: week,
			}
		}
		userStats.Weeks[week].Additions += stat.Additions
		userStats.Weeks[week].Deletions += stat.Deletions
		userStats.Weeks[week].Commits += stat.Commits
		userStats.Weeks[week].ChangedFiles += stat.ChangedFiles
		userStats.TotalCommits += stat.Commits
	}

	totalWeeks, err := GetContributionsOverTime(ctx,
		repo, start, end,
		contribution_model.RepoStatCommits,
		contribution_model.RepoStatAdditions,
		contribution_model.RepoStatDeletions,
		contribution_model.RepoStatChangedFiles,
	)
	if err != nil {
		return nil, err
	}

	var totalCommits int64
	for _, stat := range totalWeeks {
		totalCommits += stat.Commits
	}

	contributorsCommitStats["total"].Weeks = totalWeeks
	contributorsCommitStats["total"].TotalCommits = totalCommits

	return limitContributorStats(contributorsCommitStats, limit), nil
}

func limitContributorStats(contributors map[string]*ContributorData, limit int) map[string]*ContributorData {
	if limit <= 0 {
		return contributors
	}

	total := contributors["total"]

	ordered := make([]struct {
		key  string
		data *ContributorData
	}, 0, len(contributors))
	for key, data := range contributors {
		if key == "total" {
			continue
		}
		ordered = append(ordered, struct {
			key  string
			data *ContributorData
		}{
			key:  key,
			data: data,
		})
	}

	slices.SortFunc(ordered, func(a, b struct {
		key  string
		data *ContributorData
	},
	) int {
		if a.data.TotalCommits != b.data.TotalCommits {
			if a.data.TotalCommits > b.data.TotalCommits {
				return -1
			}
			return 1
		}
		return strings.Compare(a.key, b.key)
	})

	if limit > len(ordered) {
		limit = len(ordered)
	}

	filtered := make(map[string]*ContributorData, limit+1)
	filtered["total"] = total
	for _, entry := range ordered[:limit] {
		filtered[entry.key] = entry.data
	}

	return filtered
}

/*
---
82bfde2a37
author_name
abc@example.com
2026-04-16 04:07:57 +0800

 9902 files changed, 2034198 insertions(+), 298800 deletions(-)

---
2644bb8490
author_name2
abcd@example.com
*/

var errEndOfGitLogOutput = errors.New("end of git log output")

func scanOneStat(scanner *bufio.Scanner) (commitID, authorName, email string, date *time.Time, additions, deletions, changedFiles int64, err error) {
	var l string
	for scanner.Scan() {
		l = strings.TrimSpace(scanner.Text())
		if l == "---" {
			break
		}
	}

	if l != "---" {
		err = errEndOfGitLogOutput
		return commitID, authorName, email, date, additions, deletions, changedFiles, err
	}

	scanner.Scan()
	commitID = strings.TrimSpace(scanner.Text())
	scanner.Scan()
	authorName = strings.TrimSpace(scanner.Text())
	scanner.Scan()
	email = strings.TrimSpace(scanner.Text())
	scanner.Scan()
	dateStr := strings.TrimSpace(scanner.Text())
	parsedDate, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		err = fmt.Errorf("parsing date %q: %w", dateStr, err)
		return commitID, authorName, email, date, additions, deletions, changedFiles, err
	}
	date = &parsedDate
	scanner.Scan() // blank line

	if scanner.Scan() {
		numFiles, totalAdditions, totalDeletions, err := gitrepo.ParseDiffStat(scanner.Text())
		if err != nil {
			return commitID, authorName, email, date, additions, deletions, changedFiles, err
		}

		additions = int64(totalAdditions)
		deletions = int64(totalDeletions)
		changedFiles = int64(numFiles)

		scanner.Scan() // blank line
	}
	return commitID, authorName, email, date, additions, deletions, changedFiles, scanner.Err()
}

// getExtendedCommitStats returns stats for commits between start and end.
func getExtendedCommitStats(ctx context.Context, repo *repo_model.Repository, revisionRange string) ([]*ExtendedCommitStats, error) {
	if revisionRange == "" {
		return nil, nil
	}

	gitCmd := gitcmd.NewCommand("log", "--shortstat", "--no-merges", "--pretty=format:---%n%h%n%aN%n%aE%n%aI%n").
		AddDynamicArguments(revisionRange)

	stdoutReader, stdoutReaderClose := gitCmd.MakeStdoutPipe()
	defer stdoutReaderClose()

	var stats []*ExtendedCommitStats
	if err := gitrepo.RunCmdWithStderr(ctx, repo, gitCmd.
		WithPipelineFunc(func(ctx gitcmd.Context) error {
			scanner := bufio.NewScanner(stdoutReader)
			scanner.Split(bufio.ScanLines)

			for {
				_, authorName, email, commitDate, additions, deletions, changedFiles, err := scanOneStat(scanner)
				if err != nil {
					if errors.Is(err, errEndOfGitLogOutput) {
						break
					}
					return fmt.Errorf("getExtendedCommitStats scan: %w", err)
				}
				if err = scanner.Err(); err != nil {
					return fmt.Errorf("getExtendedCommitStats scan: %w", err)
				}

				stats = append(stats, &ExtendedCommitStats{
					Author: &api.CommitUser{
						Identity: api.Identity{
							Name:  authorName,
							Email: email,
						},
						Date: commitDate.Format(time.RFC3339),
					},
					Stats: &api.CommitStats{Additions: int(additions), Deletions: int(deletions), ChangedFiles: int(changedFiles)},
				})
			}
			return nil
		})); err != nil {
		return nil, fmt.Errorf("getExtendedCommitStats: %w", err)
	}

	return stats, nil
}
