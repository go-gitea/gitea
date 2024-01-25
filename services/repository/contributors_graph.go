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
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	util "code.gitea.io/gitea/modules/util"

	"gitea.com/go-chi/cache"
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
	Name         string      `json:"name"`  // Display name of the contributor
	Login        string      `json:"login"` // Login name of the contributor in case it exists
	AvatarLink   string      `json:"avatar_link"`
	HomeLink     string      `json:"home_link"`
	TotalCommits int64       `json:"total_commits"`
	Weeks        []*WeekData `json:"weeks"`
}

// ExtendedCommitStats contains information for commit stats with author data
type ExtendedCommitStats struct {
	Author *api.CommitUser  `json:"author"`
	Stats  *api.CommitStats `json:"stats"`
}

// CreateWeeks converts list of sundays to list of *api.WeekData
func CreateWeeks(sundays []int64) []*WeekData {
	var weeks []*WeekData
	for _, week := range sundays {
		weeks = append(weeks, &WeekData{
			Week:      week,
			Additions: 0,
			Deletions: 0,
			Commits:   0,
		},
		)
	}
	return weeks
}

// GetContributorStats returns contributors stats for git commits for given revision or default branch
func GetContributorStats(ctx context.Context, cache cache.Cache, repo *repo_model.Repository, revision string) (map[string]*ContributorData, error) {
	// as GetContributorStats is resource intensive we cache the result
	cacheKey := fmt.Sprintf(contributorStatsCacheKey, repo.FullName(), revision)
	if !cache.IsExist(cacheKey) {
		genReady := make(chan struct{})

		// dont start multible async generations
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

	switch v := cache.Get(cacheKey).(type) {
	case error:
		return nil, v
	case map[string]*ContributorData:
		return v, nil
	default:
		return nil, fmt.Errorf("unexpected type in cache detected")
	}
}

// GetExtendedCommitStats return the list of *ExtendedCommitStats for the given revision
func GetExtendedCommitStats(repo *git.Repository, revision string /*, limit int */) ([]*ExtendedCommitStats, error) {
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

	gitCmd := git.NewCommand(repo.Ctx, "log", "--shortstat", "--no-merges", "--pretty=format:---%n%aN%n%aE%n%as", "--reverse")
	// AddOptionFormat("--max-count=%d", limit)
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
			scanner.Split(bufio.ScanLines)

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
				scanner.Scan()
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
			_ = stdoutReader.Close()
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to get ContributorsCommitStats for repository.\nError: %w\nStderr: %s", err, stderr)
	}

	return extendedCommitStats, nil
}

func generateContributorStats(genDone chan struct{}, cache cache.Cache, cacheKey string, repo *repo_model.Repository, revision string) {
	ctx := graceful.GetManager().HammerContext()

	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo.RepoPath())
	if err != nil {
		err := fmt.Errorf("OpenRepository: %w", err)
		_ = cache.Put(cacheKey, err, contributorStatsCacheTimeout)
	}
	defer closer.Close()

	if len(revision) == 0 {
		revision = repo.DefaultBranch
	}
	extendedCommitStats, err := GetExtendedCommitStats(gitRepo, revision)
	if err != nil {
		err := fmt.Errorf("ExtendedCommitStats: %w", err)
		_ = cache.Put(cacheKey, err, contributorStatsCacheTimeout)
	}

	layout := time.DateOnly
	initialCommitDate := extendedCommitStats[0].Author.Date

	startingSunday, _ := util.FindLastSundayBeforeDate(initialCommitDate)
	endingSunday, _ := util.FindFirstSundayAfterDate(time.Now().Format(layout))

	sundays, _ := util.ListSundaysBetween(startingSunday, endingSunday)

	unknownUserAvatarLink := user_model.NewGhostUser().AvatarLink(ctx)
	contributorsCommitStats := make(map[string]*ContributorData)
	contributorsCommitStats["total"] = &ContributorData{
		Name:       "Total",
		AvatarLink: unknownUserAvatarLink,
		Weeks:      CreateWeeks(sundays),
	}
	total := contributorsCommitStats["total"]

	for _, v := range extendedCommitStats {
		userEmail := v.Author.Email
		if len(userEmail) == 0 {
			continue
		}
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
					avatarLink = unknownUserAvatarLink
				}
				contributorsCommitStats[userEmail] = &ContributorData{
					Name:       v.Author.Name,
					AvatarLink: avatarLink,
					Weeks:      CreateWeeks(sundays),
				}
			} else {
				contributorsCommitStats[userEmail] = &ContributorData{
					Name:       u.DisplayName(),
					Login:      u.LowerName,
					AvatarLink: u.AvatarLink(ctx),
					HomeLink:   u.HomeLink(),
					Weeks:      CreateWeeks(sundays),
				}
			}
		}
		// Update user statistics
		user := contributorsCommitStats[userEmail]
		startingOfWeek, _ := util.FindLastSundayBeforeDate(v.Author.Date)

		val, _ := time.Parse(layout, startingOfWeek)
		startingSundayParsed, _ := time.Parse(layout, startingSunday)
		idx := int(val.Sub(startingSundayParsed).Hours()/24) / 7

		if idx >= 0 && idx < len(user.Weeks) {
			user.Weeks[idx].Additions += v.Stats.Additions
			user.Weeks[idx].Deletions += v.Stats.Deletions
			user.Weeks[idx].Commits++
			user.TotalCommits++

			// Update overall statistics
			total.Weeks[idx].Additions += v.Stats.Additions
			total.Weeks[idx].Deletions += v.Stats.Deletions
			total.Weeks[idx].Commits++
			total.TotalCommits++
		} else {
			log.Warn("date range of the commit is not between starting date and ending date, skipping...")
		}
	}

	_ = cache.Put(cacheKey, contributorsCommitStats, contributorStatsCacheTimeout)
	generateLock.Delete(cacheKey)
	genDone <- struct{}{}
}
