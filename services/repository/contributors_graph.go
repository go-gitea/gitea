// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/avatars"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

var ErrAwaitGeneration = errors.New("contributor stats generation in progress")

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

// ExtendedCommitStats contains information for commit stats with author data
type ExtendedCommitStats struct {
	Author *api.CommitUser  `json:"author"`
	Stats  *api.CommitStats `json:"stats"`
}

// GetContributorStats returns contributors stats for git commits for given revision or default branch
func GetContributorStats(ctx context.Context, _ cache.StringCache, repo *repo_model.Repository, revision string) (map[string]*ContributorData, error) {
	if repo == nil {
		return map[string]*ContributorData{}, nil
	}
	if revision != "" && revision != repo.DefaultBranch {
		log.Debug("Contributor stats ignore revision %s for %s", revision, repo.FullName())
	}

	stats, err := repo_model.GetRepoContributorDailyStats(ctx, repo.ID)
	if err != nil {
		return nil, err
	}
	if len(stats) == 0 {
		if err := RequestContributorStatsRebuild(ctx, repo.ID); err != nil && !errors.Is(err, ErrAwaitGeneration) {
			return nil, err
		}
		return nil, ErrAwaitGeneration
	}

	unknownUserAvatarLink := user_model.NewGhostUser().AvatarLinkWithSize(ctx, 0)
	contributorsCommitStats := make(map[string]*ContributorData)
	contributorsCommitStats["total"] = &ContributorData{
		Name:  "Total",
		Weeks: make(map[int64]*WeekData),
	}
	total := contributorsCommitStats["total"]

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
					Weeks:      make(map[int64]*WeekData),
				}
			} else {
				contributorsCommitStats[email] = &ContributorData{
					Name:       user.DisplayName(),
					Login:      user.LowerName,
					AvatarLink: user.AvatarLinkWithSize(ctx, 0),
					HomeLink:   user.HomeLink(),
					Weeks:      make(map[int64]*WeekData),
				}
			}
		}
		userStats := contributorsCommitStats[email]
		week := weekStartUnixMilliFromDayStart(stat.DayStart)

		if userStats.Weeks[week] == nil {
			userStats.Weeks[week] = &WeekData{
				Week: week,
			}
		}
		if total.Weeks[week] == nil {
			total.Weeks[week] = &WeekData{
				Week: week,
			}
		}

		userStats.Weeks[week].Additions += int(stat.Additions)
		userStats.Weeks[week].Deletions += int(stat.Deletions)
		userStats.Weeks[week].Commits += int(stat.Commits)
		userStats.TotalCommits += stat.Commits

		total.Weeks[week].Additions += int(stat.Additions)
		total.Weeks[week].Deletions += int(stat.Deletions)
		total.Weeks[week].Commits += int(stat.Commits)
		total.TotalCommits += stat.Commits
	}

	return contributorsCommitStats, nil
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
