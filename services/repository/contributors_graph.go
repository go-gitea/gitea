// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models/avatars"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

var ErrAwaitGeneration = errors.New("contributor stats generation in progress")

// ContributorData represents statistical git commit count data
type ContributorData struct {
	Name         string                         `json:"name"`  // Display name of the contributor
	Login        string                         `json:"login"` // Login name of the contributor in case it exists
	AvatarLink   string                         `json:"avatar_link"`
	HomeLink     string                         `json:"home_link"`
	TotalCommits int64                          `json:"total_commits"`
	Weeks        map[int64]*repo_model.WeekData `json:"weeks"`
}

// ExtendedCommitStats contains information for commit stats with author data
type ExtendedCommitStats struct {
	Author *api.CommitUser  `json:"author"`
	Stats  *api.CommitStats `json:"stats"`
}

// GetContributorStats returns contributors stats for git commits for given revision or default branch.
func GetContributorStats(ctx context.Context, repo *repo_model.Repository, limit int, start, end *time.Time) (map[string]*ContributorData, error) {
	var startDay, endDay *repo_model.ContributorDayStart
	if start != nil {
		value := repo_model.NewContributorDayStart(start.UTC())
		startDay = &value
	}
	if end != nil {
		value := repo_model.NewContributorDayStart(end.UTC())
		endDay = &value
	}
	stats, err := repo_model.GetRepoContributorDailyStatsRange(ctx, repo.ID, startDay, endDay)
	if err != nil {
		return nil, err
	}
	hasStats := len(stats) > 0
	if !hasStats && (startDay != nil || endDay != nil) {
		hasStats, err = repo_model.HasRepoContributorDailyStats(ctx, repo.ID)
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
					Weeks:      make(map[int64]*repo_model.WeekData),
				}
			} else {
				contributorsCommitStats[email] = &ContributorData{
					Name:       user.DisplayName(),
					Login:      user.LowerName,
					AvatarLink: user.AvatarLinkWithSize(ctx, 0),
					HomeLink:   user.HomeLink(),
					Weeks:      make(map[int64]*repo_model.WeekData),
				}
			}
		}
		userStats := contributorsCommitStats[email]
		week := weekStartUnixMilliFromDayStart(stat.DayStart)

		if userStats.Weeks[week] == nil {
			userStats.Weeks[week] = &repo_model.WeekData{
				Week: week,
			}
		}
		userStats.Weeks[week].Additions += stat.Additions
		userStats.Weeks[week].Deletions += stat.Deletions
		userStats.Weeks[week].Commits += stat.Commits
		userStats.TotalCommits += stat.Commits
	}

	totalWeeks, err := GetContributionsOverTime(ctx,
		repo, start, end,
		repo_model.RepoStatCommits,
		repo_model.RepoStatAdditions,
		repo_model.RepoStatDeletions,
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

// getExtendedCommitStats return the list of *ExtendedCommitStats for the given revision
func getExtendedCommitStats(repo *git.Repository, revision string /*, limit int */) ([]*ExtendedCommitStats, error) {
	baseCommit, err := repo.GetCommit(revision)
	if err != nil {
		return nil, err
	}

	gitCmd := gitcmd.NewCommand("log", "--shortstat", "--no-merges", "--pretty=format:---%n%aN%n%aE%n%aI", "--reverse")
	// AddOptionFormat("--max-count=%d", limit)
	gitCmd.AddDynamicArguments(baseCommit.ID.String())

	stdoutReader, stdoutReaderClose := gitCmd.MakeStdoutPipe()
	defer stdoutReaderClose()

	var extendedCommitStats []*ExtendedCommitStats
	err = gitCmd.WithDir(repo.Path).
		WithPipelineFunc(func(ctx gitcmd.Context) error {
			scanner := bufio.NewScanner(stdoutReader)

			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "---" {
					continue
				}
				scanner.Scan()
				authorName := strings.TrimSpace(scanner.Text())
				scanner.Scan()
				authorEmail := strings.TrimSpace(scanner.Text())
				scanner.Scan()
				date := strings.TrimSpace(scanner.Text())
				scanner.Scan()
				stats := strings.TrimSpace(scanner.Text())
				if authorName == "" || authorEmail == "" || date == "" || stats == "" {
					// FIXME: find a better way to parse the output so that we will handle this properly
					log.Warn("Something is wrong with git log output, skipping...")
					log.Warn("authorName: %s,  authorEmail: %s,  date: %s,  stats: %s", authorName, authorEmail, date, stats)
					continue
				}
				//  1 file changed, 1 insertion(+), 1 deletion(-)
				fields := strings.Split(stats, ",")

				commitStats := api.CommitStats{}
				for _, field := range fields[1:] {
					parts := strings.Split(strings.TrimSpace(field), " ")
					value, contributionType := parts[0], parts[1]
					amount, _ := strconv.Atoi(value)

					if strings.HasPrefix(contributionType, "insertion") {
						commitStats.Additions = amount
					} else {
						commitStats.Deletions = amount
					}
				}
				commitStats.Total = commitStats.Additions + commitStats.Deletions
				scanner.Text() // empty line at the end

				res := &ExtendedCommitStats{
					Author: &api.CommitUser{
						Identity: api.Identity{
							Name:  authorName,
							Email: authorEmail,
						},
						Date: date,
					},
					Stats: &commitStats,
				}
				extendedCommitStats = append(extendedCommitStats, res)
			}
			return nil
		}).
		RunWithStderr(repo.Ctx)
	if err != nil {
		return nil, fmt.Errorf("ContributorsCommitStats: %w", err)
	}

	return extendedCommitStats, nil
}

// getExtendedCommitStatsRange returns stats for commits between start and end.
func getExtendedCommitStatsRange(repo *git.Repository, startCommitID, endCommitID string) ([]*ExtendedCommitStats, error) {
	if endCommitID == "" {
		return nil, nil
	}
	objectFormat, err := repo.GetObjectFormat()
	if err != nil {
		return nil, err
	}
	commitRange := endCommitID
	if startCommitID != "" && startCommitID != objectFormat.EmptyObjectID().String() {
		commitRange = fmt.Sprintf("%s..%s", startCommitID, endCommitID)
	}

	gitCmd := gitcmd.NewCommand("log", "--shortstat", "--no-merges", "--pretty=format:---%n%aN%n%aE%n%aI", "--reverse")
	gitCmd.AddDynamicArguments(commitRange)

	stdoutReader, stdoutReaderClose := gitCmd.MakeStdoutPipe()
	defer stdoutReaderClose()

	var extendedCommitStats []*ExtendedCommitStats
	if err := gitCmd.WithDir(repo.Path).
		WithPipelineFunc(func(ctx gitcmd.Context) error {
			scanner := bufio.NewScanner(stdoutReader)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "---" {
					continue
				}
				scanner.Scan()
				authorName := strings.TrimSpace(scanner.Text())
				scanner.Scan()
				authorEmail := strings.TrimSpace(scanner.Text())
				scanner.Scan()
				date := strings.TrimSpace(scanner.Text())
				scanner.Scan()
				stats := strings.TrimSpace(scanner.Text())
				if authorName == "" || authorEmail == "" || date == "" || stats == "" {
					log.Warn("Something is wrong with git log output, skipping...")
					log.Warn("authorName: %s,  authorEmail: %s,  date: %s,  stats: %s", authorName, authorEmail, date, stats)
					continue
				}

				fields := strings.Split(stats, ",")
				commitStats := api.CommitStats{}
				for _, field := range fields[1:] {
					parts := strings.Split(strings.TrimSpace(field), " ")
					if len(parts) < 2 {
						continue
					}
					value, contributionType := parts[0], parts[1]
					amount, _ := strconv.Atoi(value)
					if strings.HasPrefix(contributionType, "insertion") {
						commitStats.Additions = amount
					} else {
						commitStats.Deletions = amount
					}
				}
				commitStats.Total = commitStats.Additions + commitStats.Deletions
				scanner.Text()

				res := &ExtendedCommitStats{
					Author: &api.CommitUser{
						Identity: api.Identity{
							Name:  authorName,
							Email: authorEmail,
						},
						Date: date,
					},
					Stats: &commitStats,
				}
				extendedCommitStats = append(extendedCommitStats, res)
			}
			return nil
		}).
		RunWithStderr(repo.Ctx); err != nil {
		return nil, fmt.Errorf("ContributorsCommitStatsRange: %w", err)
	}

	return extendedCommitStats, nil
}
