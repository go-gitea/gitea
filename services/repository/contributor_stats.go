// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/timeutil"
)

const (
	contributorDayMillis  = int64(24 * time.Hour / time.Millisecond)
	contributorWeekMillis = 7 * contributorDayMillis
)

type ContributorStatsUpdateOptions struct {
	RepoID      int64
	OldCommitID string
	NewCommitID string
}

type ContributorStatsRebuildOptions struct {
	RepoID int64
}

var (
	contributorStatsUpdateQueue  *queue.WorkerPoolQueue[*ContributorStatsUpdateOptions]
	contributorStatsRebuildQueue *queue.WorkerPoolQueue[*ContributorStatsRebuildOptions]
)

func initContributorStatsQueue(ctx context.Context) error {
	contributorStatsUpdateQueue = queue.CreateSimpleQueue(ctx, "contributor_stats_update", handlerContributorStatsUpdate)
	if contributorStatsUpdateQueue == nil {
		return errors.New("unable to create contributor_stats_update queue")
	}
	go graceful.GetManager().RunWithCancel(contributorStatsUpdateQueue)

	contributorStatsRebuildQueue = queue.CreateUniqueQueue(ctx, "contributor_stats_rebuild", handlerContributorStatsRebuild)
	if contributorStatsRebuildQueue == nil {
		return errors.New("unable to create contributor_stats_rebuild queue")
	}
	go graceful.GetManager().RunWithCancel(contributorStatsRebuildQueue)
	return nil
}

func handlerContributorStatsUpdate(items ...*ContributorStatsUpdateOptions) []*ContributorStatsUpdateOptions {
	ctx := graceful.GetManager().HammerContext()
	for _, opts := range items {
		if err := processContributorStatsUpdate(ctx, opts); err != nil && !errors.Is(err, ErrAwaitGeneration) {
			if context.Cause(ctx) == context.Canceled {
				log.Warn("contributor stats update canceled for repo %d", opts.RepoID)
				return nil
			}
			log.Error("contributor stats update failed for repo %d: %v", opts.RepoID, err)
		}
	}
	return nil
}

func handlerContributorStatsRebuild(items ...*ContributorStatsRebuildOptions) []*ContributorStatsRebuildOptions {
	ctx := graceful.GetManager().HammerContext()
	for _, opts := range items {
		if err := processContributorStatsRebuild(ctx, opts); err != nil && !errors.Is(err, ErrAwaitGeneration) {
			if context.Cause(ctx) == context.Canceled {
				log.Warn("contributor stats rebuild canceled for repo %d", opts.RepoID)
				return nil
			}
			log.Error("contributor stats rebuild failed for repo %d: %v", opts.RepoID, err)
		}
	}
	return nil
}

func enqueueContributorStatsUpdate(repoID int64, oldCommitID, newCommitID string) error {
	if contributorStatsUpdateQueue == nil {
		return nil
	}
	return contributorStatsUpdateQueue.Push(&ContributorStatsUpdateOptions{
		RepoID:      repoID,
		OldCommitID: oldCommitID,
		NewCommitID: newCommitID,
	})
}

func enqueueContributorStatsRebuild(repoID int64) error {
	if contributorStatsRebuildQueue == nil {
		return nil
	}
	if err := contributorStatsRebuildQueue.Push(&ContributorStatsRebuildOptions{RepoID: repoID}); err != nil {
		if errors.Is(err, queue.ErrAlreadyInQueue) {
			return ErrAwaitGeneration
		}
		return err
	}
	return nil
}

func markRepoContributorStatsDirty(ctx context.Context, repoID int64) error {
	meta, err := repo_model.EnsureRepoContributorMeta(ctx, repoID)
	if err != nil {
		return err
	}
	meta.Dirty = true
	meta.LastProcessedCommitID = ""
	meta.UpdatedUnix = timeutil.TimeStampNow()
	return repo_model.UpdateRepoContributorMeta(ctx, meta, "dirty", "last_processed_commit_id", "updated_unix")
}

// RequestContributorStatsRebuild triggers a rebuild of contributor statistics.
func RequestContributorStatsRebuild(ctx context.Context, repoID int64) error {
	if err := markRepoContributorStatsDirty(ctx, repoID); err != nil {
		return err
	}
	return enqueueContributorStatsRebuild(repoID)
}

// RebuildMissingContributorStats enqueues rebuild tasks for repositories missing stats.
func RebuildMissingContributorStats(ctx context.Context) error {
	var errs []error
	if err := repo_model.IterateRepoIDsWithoutContributorDaily(ctx, 200, func(repoIDs []int64) error {
		for _, repoID := range repoIDs {
			if err := RequestContributorStatsRebuild(ctx, repoID); err != nil && !errors.Is(err, ErrAwaitGeneration) {
				errs = append(errs, fmt.Errorf("rebuild contributor stats failed for repo %d: %w", repoID, err))
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return errors.Join(errs...)
}

func processContributorStatsUpdate(ctx context.Context, opts *ContributorStatsUpdateOptions) error {
	if opts.NewCommitID == "" {
		return nil
	}

	repo, err := repo_model.GetRepositoryByID(ctx, opts.RepoID)
	if err != nil {
		return err
	}
	if repo.IsEmpty || repo.DefaultBranch == "" {
		return nil
	}

	meta, err := repo_model.EnsureRepoContributorMeta(ctx, repo.ID)
	if err != nil {
		return err
	}
	if meta.Dirty {
		return nil
	}

	startCommitID := meta.LastProcessedCommitID
	if startCommitID == "" {
		startCommitID = opts.OldCommitID
	}
	if startCommitID == "" {
		return RequestContributorStatsRebuild(ctx, repo.ID)
	}

	gitRepo, err := gitrepo.OpenRepository(ctx, repo)
	if err != nil {
		return err
	}
	defer gitRepo.Close()

	newCommit, err := gitRepo.GetCommit(opts.NewCommitID)
	if err != nil {
		return err
	}
	isForcePush, err := newCommit.IsForcePush(startCommitID)
	if err != nil {
		return err
	}
	if isForcePush {
		return RequestContributorStatsRebuild(ctx, repo.ID)
	}

	updates, err := collectContributorDailyUpdates(ctx, repo, gitRepo, startCommitID, opts.NewCommitID)
	if err != nil {
		return err
	}
	if err := repo_model.ApplyRepoContributorDailyUpdates(ctx, updates); err != nil {
		return err
	}

	meta.LastProcessedCommitID = opts.NewCommitID
	meta.Dirty = false
	meta.UpdatedUnix = timeutil.TimeStampNow()
	return repo_model.UpdateRepoContributorMeta(ctx, meta, "last_processed_commit_id", "dirty", "updated_unix")
}

func processContributorStatsRebuild(ctx context.Context, opts *ContributorStatsRebuildOptions) error {
	repo, err := repo_model.GetRepositoryByID(ctx, opts.RepoID)
	if err != nil {
		return err
	}
	if repo.IsEmpty || repo.DefaultBranch == "" {
		return nil
	}

	gitRepo, err := gitrepo.OpenRepository(ctx, repo)
	if err != nil {
		return err
	}
	defer gitRepo.Close()

	headCommit, err := gitRepo.GetCommit(repo.DefaultBranch)
	if err != nil {
		return err
	}

	stats, err := getExtendedCommitStats(gitRepo, repo.DefaultBranch)
	if err != nil {
		return err
	}

	updates, err := buildContributorDailyUpdates(ctx, repo, stats)
	if err != nil {
		return err
	}
	dailyStats := make([]*repo_model.ContributorDaily, 0, len(updates))
	now := timeutil.TimeStampNow()
	for _, update := range updates {
		dailyStats = append(dailyStats, &repo_model.ContributorDaily{
			RepoID:      update.RepoID,
			DayStart:    update.DayStart,
			UserID:      update.UserID,
			Email:       update.Email,
			AuthorName:  update.AuthorName,
			Additions:   update.Additions,
			Deletions:   update.Deletions,
			Commits:     update.Commits,
			UpdatedUnix: now,
		})
	}

	if err := repo_model.ReplaceRepoContributorDailyStats(ctx, repo.ID, dailyStats); err != nil {
		return err
	}

	meta, err := repo_model.EnsureRepoContributorMeta(ctx, repo.ID)
	if err != nil {
		return err
	}
	meta.LastProcessedCommitID = headCommit.ID.String()
	meta.Dirty = false
	meta.UpdatedUnix = timeutil.TimeStampNow()
	return repo_model.UpdateRepoContributorMeta(ctx, meta, "last_processed_commit_id", "dirty", "updated_unix")
}

func collectContributorDailyUpdates(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, startCommitID, endCommitID string) ([]*repo_model.ContributorDailyUpdate, error) {
	stats, err := getExtendedCommitStatsRange(gitRepo, startCommitID, endCommitID)
	if err != nil {
		return nil, err
	}
	return buildContributorDailyUpdates(ctx, repo, stats)
}

func buildContributorDailyUpdates(ctx context.Context, repo *repo_model.Repository, stats []*ExtendedCommitStats) ([]*repo_model.ContributorDailyUpdate, error) {
	if len(stats) == 0 {
		return nil, nil
	}

	updates := make(map[contributorDailyKey]*repo_model.ContributorDailyUpdate)
	userCache := make(map[string]*user_model.User)

	for _, stat := range stats {
		if stat.Author == nil || stat.Stats == nil {
			continue
		}
		email := strings.ToLower(strings.TrimSpace(stat.Author.Email))
		if email == "" {
			continue
		}

		authorTime, err := time.Parse(time.RFC3339, stat.Author.Date)
		if err != nil {
			return nil, err
		}
		dayStart := dayStartUnixMilli(authorTime)

		user, ok := userCache[email]
		if !ok {
			user, _ = user_model.GetUserByEmail(ctx, email)
			userCache[email] = user
		}

		key := contributorDailyKey{
			dayStart: dayStart,
			email:    email,
		}
		if user != nil {
			key.userID = user.ID
			key.email = ""
		}

		update := updates[key]
		if update == nil {
			update = &repo_model.ContributorDailyUpdate{
				RepoID:     repo.ID,
				DayStart:   dayStart,
				UserID:     key.userID,
				Email:      email,
				AuthorName: stat.Author.Name,
			}
			updates[key] = update
		}
		update.Additions += int64(stat.Stats.Additions)
		update.Deletions += int64(stat.Stats.Deletions)
		update.Commits++
	}

	res := make([]*repo_model.ContributorDailyUpdate, 0, len(updates))
	for _, update := range updates {
		res = append(res, update)
	}
	return res, nil
}

type contributorDailyKey struct {
	dayStart repo_model.ContributorDayStart
	userID   int64
	email    string
}

func dayStartUnixMilli(t time.Time) repo_model.ContributorDayStart {
	return repo_model.NewContributorDayStart(t)
}

func weekStartUnixMilliFromDayStart(dayStart repo_model.ContributorDayStart) int64 {
	day := time.UnixMilli(dayStart.UnixMilli()).UTC()
	daysToSubtract := int(day.Weekday())
	return day.AddDate(0, 0, -daysToSubtract).UnixMilli()
}

func weekStartUnixMilliFromTime(t time.Time) int64 {
	return weekStartUnixMilliFromDayStart(dayStartUnixMilli(t))
}
