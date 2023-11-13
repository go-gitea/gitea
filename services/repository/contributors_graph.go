// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"fmt"
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

// CreateWeeks converts list of sundays to list of *api.WeekData
func CreateWeeks(sundays []int64) []*api.WeekData {
	var weeks []*api.WeekData
	for _, week := range sundays {
		weeks = append(weeks, &api.WeekData{
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
func GetContributorStats(ctx context.Context, cache cache.Cache, repo *repo_model.Repository, revision string) (map[string]*api.ContributorData, error) {
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
	case map[string]*api.ContributorData:
		return v, nil
	default:
		return nil, fmt.Errorf("unexpected type in cache detected")
	}
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
	extendedCommitStats, err := gitRepo.ExtendedCommitStats(revision)
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
	contributorsCommitStats := make(map[string]*api.ContributorData)
	contributorsCommitStats["total"] = &api.ContributorData{
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
				contributorsCommitStats[userEmail] = &api.ContributorData{
					Name:       v.Author.Name,
					AvatarLink: avatarLink,
					Weeks:      CreateWeeks(sundays),
				}
			} else {
				contributorsCommitStats[userEmail] = &api.ContributorData{
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
