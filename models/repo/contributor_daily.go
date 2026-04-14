// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// ContributorDayStart represents the start of a day in UTC as Unix milliseconds.
type ContributorDayStart int64

// NewContributorDayStart returns the day start for the given time in UTC.
func NewContributorDayStart(t time.Time) ContributorDayStart {
	t = t.UTC()
	return ContributorDayStart(time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).UnixMilli())
}

// UnixMilli returns the day start as Unix milliseconds.
func (d ContributorDayStart) UnixMilli() int64 {
	return int64(d)
}

// ContributorDaily stores per-day contributor stats for a repository.
type ContributorDaily struct {
	ID          int64               `xorm:"pk autoincr"`
	RepoID      int64               `xorm:"UNIQUE(repo_user_day) INDEX NOT NULL"`
	DayStart    ContributorDayStart `xorm:"UNIQUE(repo_user_day) INDEX NOT NULL"`
	UserID      int64               `xorm:"UNIQUE(repo_user_day) INDEX NOT NULL DEFAULT 0"`
	Email       string              `xorm:"UNIQUE(repo_user_day) INDEX VARCHAR(255) NOT NULL DEFAULT ''"`
	AuthorName  string              `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	Additions   int64               `xorm:"NOT NULL DEFAULT 0"`
	Deletions   int64               `xorm:"NOT NULL DEFAULT 0"`
	Commits     int64               `xorm:"NOT NULL DEFAULT 0"`
	UpdatedUnix timeutil.TimeStamp  `xorm:"INDEX updated"`
}

func (ContributorDaily) TableName() string {
	return "repo_contributor_daily"
}

// ContributorDailyUpdate is an increment applied to ContributorDaily.
type ContributorDailyUpdate struct {
	RepoID     int64
	DayStart   ContributorDayStart
	UserID     int64
	Email      string
	AuthorName string
	Additions  int64
	Deletions  int64
	Commits    int64
}

// ContributorSummary represents total commit counts per contributor.
type ContributorSummary struct {
	UserID     int64
	Email      string
	AuthorName string
	Commits    int64
}

func init() {
	db.RegisterModel(new(ContributorDaily))
}

// GetRepoContributorDailyStats returns all daily stats for a repository.
func GetRepoContributorDailyStats(ctx context.Context, repoID int64) ([]*ContributorDaily, error) {
	stats := make([]*ContributorDaily, 0, 512)
	return stats, db.GetEngine(ctx).
		Where("repo_id = ?", repoID).
		Asc("day_start").
		Find(&stats)
}

// IterateRepoIDsWithoutContributorDaily iterates repo IDs without contributor daily stats.
func IterateRepoIDsWithoutContributorDaily(ctx context.Context, batchSize int, handle func([]int64) error) error {
	if batchSize <= 0 {
		batchSize = 200
	}
	lastID := int64(0)
	for {
		repoIDs := make([]int64, 0, batchSize)
		if err := db.GetEngine(ctx).
			SQL("SELECT id FROM repository WHERE is_empty = ? AND id > ? AND NOT EXISTS (SELECT 1 FROM repo_contributor_daily WHERE repo_id = repository.id) ORDER BY id LIMIT ?", false, lastID, batchSize).
			Find(&repoIDs); err != nil {
			return err
		}
		if len(repoIDs) == 0 {
			return nil
		}
		if err := handle(repoIDs); err != nil {
			return err
		}
		lastID = repoIDs[len(repoIDs)-1]
	}
}

// GetRepoTopContributors returns the top contributors and total contributor count.
func GetRepoTopContributors(ctx context.Context, repoID int64, limit int) ([]*ContributorSummary, int64, error) {
	contributors := make([]*ContributorSummary, 0, limit)
	if limit <= 0 {
		return contributors, 0, nil
	}
	if err := db.GetEngine(ctx).
		Table("repo_contributor_daily").
		Select("user_id, email, max(author_name) as author_name, sum(commits) as commits").
		Where("repo_id = ?", repoID).
		GroupBy("user_id, email").
		Desc("commits").
		Limit(limit).
		Find(&contributors); err != nil {
		return nil, 0, err
	}

	var count struct {
		Total int64 `xorm:"total"`
	}
	_, err := db.GetEngine(ctx).
		SQL("SELECT COUNT(*) AS total FROM (SELECT user_id, email FROM repo_contributor_daily WHERE repo_id=? GROUP BY user_id, email) temp", repoID).
		Get(&count)
	if err != nil {
		return contributors, 0, err
	}
	return contributors, count.Total, nil
}

// DeleteRepoContributorDailyStats removes all daily stats for a repository.
func DeleteRepoContributorDailyStats(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Delete(&ContributorDaily{})
	return err
}

// ReplaceRepoContributorDailyStats replaces all daily stats for a repository.
func ReplaceRepoContributorDailyStats(ctx context.Context, repoID int64, stats []*ContributorDaily) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := DeleteRepoContributorDailyStats(ctx, repoID); err != nil {
			return err
		}
		if len(stats) == 0 {
			return nil
		}
		batchSize := db.MaxBatchInsertSize(new(ContributorDaily))
		for i := 0; i < len(stats); i += batchSize {
			end := min(i+batchSize, len(stats))
			if _, err := db.GetEngine(ctx).Insert(stats[i:end]); err != nil {
				return err
			}
		}
		return nil
	})
}

// ApplyRepoContributorDailyUpdates applies incremental updates for a repository.
func ApplyRepoContributorDailyUpdates(ctx context.Context, updates []*ContributorDailyUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		now := timeutil.TimeStampNow()
		for _, update := range updates {
			if update.Email == "" && update.UserID == 0 {
				continue
			}
			update.Email = strings.ToLower(update.Email)
			updated, err := db.GetEngine(ctx).
				Where("repo_id = ? AND day_start = ? AND user_id = ? AND email = ?", update.RepoID, update.DayStart, update.UserID, update.Email).
				Incr("additions", update.Additions).
				Incr("deletions", update.Deletions).
				Incr("commits", update.Commits).
				Cols("updated_unix", "author_name").
				Update(&ContributorDaily{
					UpdatedUnix: now,
					AuthorName:  update.AuthorName,
				})
			if err != nil {
				return err
			}
			if updated > 0 {
				continue
			}

			record := &ContributorDaily{
				RepoID:      update.RepoID,
				DayStart:    update.DayStart,
				UserID:      update.UserID,
				Email:       update.Email,
				AuthorName:  update.AuthorName,
				Additions:   update.Additions,
				Deletions:   update.Deletions,
				Commits:     update.Commits,
				UpdatedUnix: now,
			}
			if _, err := db.GetEngine(ctx).Insert(record); err != nil {
				return err
			}
		}
		return nil
	})
}
