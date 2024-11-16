// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/models/avatars"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
)

const (
	contributorStatsCacheKey           = "GetContributorStats/%s/%s"
	contributorStatsCacheTimeout int64 = 60 * 10
)

var (
	ErrAwaitGeneration  = errors.New("generation took longer than ")
	awaitGenerationTime = time.Second * 5
	generateLock        = sync.Map{}
)

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

// ExtendedCommitStats contains information for commit stats with both author and coauthors data
type ExtendedCommitStats struct {
	Author    *api.CommitUser   `json:"author"`
	CoAuthors []*api.CommitUser `json:"co_authors"`
	Stats     *api.CommitStats  `json:"stats"`
}

const layout = time.DateOnly

func findLastSundayBeforeDate(dateStr string) (string, error) {
	date, err := time.Parse(layout, dateStr)
	if err != nil {
		return "", err
	}

	weekday := date.Weekday()
	daysToSubtract := int(weekday) - int(time.Sunday)
	if daysToSubtract < 0 {
		daysToSubtract += 7
	}

	lastSunday := date.AddDate(0, 0, -daysToSubtract)
	return lastSunday.Format(layout), nil
}

// GetContributorStats returns contributors stats for git commits for given revision or default branch
func GetContributorStats(ctx context.Context, cache cache.StringCache, repo *repo_model.Repository, revision string) (map[string]*ContributorData, error) {
	// as GetContributorStats is resource intensive we cache the result
	cacheKey := fmt.Sprintf(contributorStatsCacheKey, repo.FullName(), revision)
	if !cache.IsExist(cacheKey) {
		genReady := make(chan struct{})

		// dont start multiple async generations
		_, run := generateLock.Load(cacheKey)
		if run {
			return nil, ErrAwaitGeneration
		}

		generateLock.Store(cacheKey, struct{}{})
		// run generation async
		go generateContributorStats(genReady, cache, cacheKey, repo, revision)

		select {
		case <-time.After(awaitGenerationTime):
			return nil, ErrAwaitGeneration
		case <-genReady:
			// we got generation ready before timeout
			break
		}
	}
	// TODO: renew timeout of cache cache.UpdateTimeout(cacheKey, contributorStatsCacheTimeout)
	var res map[string]*ContributorData
	if _, cacheErr := cache.GetJSON(cacheKey, &res); cacheErr != nil {
		return nil, fmt.Errorf("cached error: %w", cacheErr.ToError())
	}
	return res, nil
}

// getExtendedCommitStats return the list of *ExtendedCommitStats for the given revision
func getExtendedCommitStats(repo *git.Repository, revision string /*, limit int */) ([]*ExtendedCommitStats, error) {
	baseCommit, err := repo.GetCommit(revision)
	if err != nil {
		return nil, err
	}
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	gitCmd := git.NewCommand(repo.Ctx, "log", "--shortstat", "--no-merges", "--pretty=format:---%n%aN%n%aE%n%as%n%(trailers:key=Co-authored-by,valueonly=true)", "--reverse")
	gitCmd.AddDynamicArguments(baseCommit.ID.String())

	var extendedCommitStats []*ExtendedCommitStats
	stderr := new(strings.Builder)
	err = gitCmd.Run(&git.RunOpts{
		Dir:    repo.Path,
		Stdout: stdoutWriter,
		Stderr: stderr,
		PipelineFunc: func(ctx context.Context, cancel context.CancelFunc) error {
			_ = stdoutWriter.Close()
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

				var coAuthors []*api.CommitUser
				emailSet := map[string]bool{}
				for scanner.Scan() {
					line := scanner.Text()
					if line == "" {
						// There should be an empty line before we read the commit stats line.
						break
					}
					coAuthorName, coAuthorEmail, err := util.ParseCommitTrailerValueWithAuthor(line)
					if authorEmail == coAuthorEmail {
						// Authors shouldn't be co-authors too.
						continue
					}
					if err != nil {
						continue
					}
					if _, exists := emailSet[coAuthorEmail]; exists {
						continue
					}
					emailSet[coAuthorEmail] = true
					coAuthor := &api.CommitUser{
						Identity: api.Identity{Name: coAuthorName, Email: coAuthorEmail},
						Date:     date,
					}
					coAuthors = append(coAuthors, coAuthor)
				}

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
					CoAuthors: coAuthors,
					Stats:     &commitStats,
				}
				extendedCommitStats = append(extendedCommitStats, res)
			}
			_ = stdoutReader.Close()
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to get ContributorsCommitStats for repository.\nError: %w\nStderr: %s", err, stderr)
	}

	return extendedCommitStats, nil
}

func generateContributorStats(genDone chan struct{}, cache cache.StringCache, cacheKey string, repo *repo_model.Repository, revision string) {
	ctx := graceful.GetManager().HammerContext()

	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		_ = cache.PutJSON(cacheKey, fmt.Errorf("OpenRepository: %w", err), contributorStatsCacheTimeout)
		return
	}
	defer closer.Close()

	if len(revision) == 0 {
		revision = repo.DefaultBranch
	}
	extendedCommitStats, err := getExtendedCommitStats(gitRepo, revision)
	if err != nil {
		_ = cache.PutJSON(cacheKey, fmt.Errorf("ExtendedCommitStats: %w", err), contributorStatsCacheTimeout)
		return
	}
	if len(extendedCommitStats) == 0 {
		_ = cache.PutJSON(cacheKey, fmt.Errorf("no commit stats returned for revision '%s'", revision), contributorStatsCacheTimeout)
		return
	}

	unknownUserAvatarLink := user_model.NewGhostUser().AvatarLinkWithSize(ctx, 0)
	contributorsCommitStats := make(map[string]*ContributorData)
	contributorsCommitStats["total"] = &ContributorData{
		Name:  "Total",
		Weeks: make(map[int64]*WeekData),
	}
	total := contributorsCommitStats["total"]

	for _, v := range extendedCommitStats {
		userEmail := v.Author.Email
		if len(userEmail) == 0 {
			continue
		}

		authorData := getContributorData(ctx, contributorsCommitStats, v.Author, unknownUserAvatarLink)
		date := v.Author.Date
		stats := v.Stats
		updateUserAndOverallStats(stats, date, authorData, total, false)

		for _, coAuthor := range v.CoAuthors {
			coAuthorData := getContributorData(ctx, contributorsCommitStats, coAuthor, unknownUserAvatarLink)
			updateUserAndOverallStats(stats, date, coAuthorData, total, true)
		}
	}

	_ = cache.PutJSON(cacheKey, contributorsCommitStats, contributorStatsCacheTimeout)
	generateLock.Delete(cacheKey)
	if genDone != nil {
		genDone <- struct{}{}
	}
}

func getContributorData(ctx context.Context, contributorsCommitStats map[string]*ContributorData, user *api.CommitUser, defaultUserAvatarLink string) *ContributorData {
	userEmail := user.Email
	u, _ := user_model.GetUserByEmail(ctx, userEmail)
	if u != nil {
		// update userEmail with user's primary email address so
		// that different mail addresses will linked to same account
		userEmail = u.GetEmail()
	}

	if _, ok := contributorsCommitStats[userEmail]; !ok {
		if u == nil {
			avatarLink := avatars.GenerateEmailAvatarFastLink(ctx, userEmail, 0)
			if avatarLink == "" {
				avatarLink = defaultUserAvatarLink
			}
			contributorsCommitStats[userEmail] = &ContributorData{
				Name:       user.Name,
				AvatarLink: avatarLink,
				Weeks:      make(map[int64]*WeekData),
			}
		} else {
			contributorsCommitStats[userEmail] = &ContributorData{
				Name:       u.DisplayName(),
				Login:      u.LowerName,
				AvatarLink: u.AvatarLinkWithSize(ctx, 0),
				HomeLink:   u.HomeLink(),
				Weeks:      make(map[int64]*WeekData),
			}
		}
	}
	return contributorsCommitStats[userEmail]
}

func updateUserAndOverallStats(stats *api.CommitStats, commitDate string, user, total *ContributorData, isCoAuthor bool) {
	startingOfWeek, _ := findLastSundayBeforeDate(commitDate)

	val, _ := time.Parse(layout, startingOfWeek)
	week := val.UnixMilli()

	if user.Weeks[week] == nil {
		user.Weeks[week] = &WeekData{
			Additions: 0,
			Deletions: 0,
			Commits:   0,
			Week:      week,
		}
	}
	if total.Weeks[week] == nil {
		total.Weeks[week] = &WeekData{
			Additions: 0,
			Deletions: 0,
			Commits:   0,
			Week:      week,
		}
	}
	// Update user statistics
	user.Weeks[week].Additions += stats.Additions
	user.Weeks[week].Deletions += stats.Deletions
	user.Weeks[week].Commits++
	user.TotalCommits++

	if isCoAuthor {
		// We would have or will count these additions/deletions/commits already when we encounter the original
		// author of the commit. Let's avoid this duplication.
		return
	}

	// Update overall statistics
	total.Weeks[week].Additions += stats.Additions
	total.Weeks[week].Deletions += stats.Deletions
	total.Weeks[week].Commits++
	total.TotalCommits++
}
