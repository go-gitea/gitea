// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package contribution

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
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
	ID           int64               `xorm:"pk autoincr"`
	RepoID       int64               `xorm:"UNIQUE(repo_user_day) INDEX NOT NULL"`
	DayStart     ContributorDayStart `xorm:"UNIQUE(repo_user_day) INDEX NOT NULL"`
	UserID       int64               `xorm:"UNIQUE(repo_user_day) INDEX NOT NULL DEFAULT 0"`
	Email        string              `xorm:"UNIQUE(repo_user_day) INDEX VARCHAR(255) NOT NULL DEFAULT ''"`
	AuthorName   string              `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	Additions    int64               `xorm:"NOT NULL DEFAULT 0"`
	Deletions    int64               `xorm:"NOT NULL DEFAULT 0"`
	Commits      int64               `xorm:"NOT NULL DEFAULT 0"`
	ChangedFiles int64               `xorm:"NOT NULL DEFAULT 0"`
	UpdatedUnix  timeutil.TimeStamp  `xorm:"INDEX updated"`
}

func (ContributorDaily) TableName() string {
	return "repo_contributor_daily"
}

// ContributorDailyUpdate is an increment applied to ContributorDaily.
type ContributorDailyUpdate struct {
	RepoID       int64
	DayStart     ContributorDayStart
	UserID       int64
	Email        string
	AuthorName   string
	Additions    int64
	Deletions    int64
	Commits      int64
	ChangedFiles int64
}

// ContributorSummary represents total commit counts per contributor.
type ContributorSummary struct {
	UserID       int64
	Email        string
	AuthorName   string
	Additions    int64
	Deletions    int64
	Commits      int64
	ChangedFiles int64
}

type WeekData struct {
	Week         int64 `json:"week"`          // Starting day of the week as Unix timestamp
	Additions    int64 `json:"additions"`     // Number of additions in that week
	Deletions    int64 `json:"deletions"`     // Number of deletions in that week
	Commits      int64 `json:"commits"`       // Number of commits in that week
	ChangedFiles int64 `json:"changed_files"` // Number of changed files in that week
}

// RepoStatType describes which weekly stats to aggregate.
type RepoStatType uint8

const (
	RepoStatAdditions RepoStatType = iota
	RepoStatDeletions
	RepoStatCommits
	RepoStatChangedFiles
)

func init() {
	db.RegisterModel(new(ContributorDaily))
}

// GetRepoContributorDailyStats returns all daily stats for a repository.
func GetRepoContributorDailyStats(ctx context.Context, repoID int64) ([]*ContributorDaily, error) {
	return GetRepoContributorDailyStatsRange(ctx, repoID, nil, nil)
}

// GetRepoContributorDailyStatsRange returns daily stats for a repository in the given day range.
func GetRepoContributorDailyStatsRange(ctx context.Context, repoID int64, start, end *ContributorDayStart) ([]*ContributorDaily, error) {
	stats := make([]*ContributorDaily, 0, 512)
	sess := db.GetEngine(ctx).
		Where("repo_id = ?", repoID)
	if start != nil {
		sess = sess.And("day_start >= ?", int64(*start))
	}
	if end != nil {
		sess = sess.And("day_start < ?", int64(*end))
	}
	return stats, sess.Asc("day_start").Find(&stats)
}

// HasRepoContributorDailyStats reports whether the repository has any contributor stats.
func HasRepoContributorDailyStats(ctx context.Context, repoID int64) (bool, error) {
	return db.GetEngine(ctx).
		Where("repo_id = ?", repoID).
		Exist(new(ContributorDaily))
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
			Table("repository").
			Select("repository.id").
			Join("LEFT", "repo_contributor_daily", "repo_contributor_daily.repo_id = repository.id").
			Where("repository.is_empty = ?", false).
			And("repository.id > ?", lastID).
			And("repo_contributor_daily.repo_id IS NULL").
			Asc("repository.id").
			Limit(batchSize).
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
	listOpts := db.ListOptions{PageSize: limit, Page: 1}
	return GetRepoContributors(ctx, repoID, true, listOpts)
}

// GetRepoContributors returns contributors for a repository with pagination.
func GetRepoContributors(ctx context.Context, repoID int64, includeAnonymous bool, listOpts db.ListOptions) ([]*ContributorSummary, int64, error) {
	contributors := make([]*ContributorSummary, 0, listOpts.PageSize)
	if listOpts.PageSize <= 0 {
		listOpts.PageSize = 20
	}

	sess := db.GetEngine(ctx).
		Table("repo_contributor_daily").
		Select("user_id, email, max(author_name) as author_name, sum(additions) as additions, sum(deletions) as deletions, sum(commits) as commits, sum(changed_files) as changed_files").
		Where("repo_id = ?", repoID)
	if !includeAnonymous {
		sess = sess.And("user_id > 0")
	}
	if err := db.SetSessionPagination(sess, &listOpts).
		GroupBy("user_id, email").
		Desc("commits").
		Find(&contributors); err != nil {
		return nil, 0, err
	}

	where := "repo_id = ?"
	args := []any{repoID}
	if !includeAnonymous {
		where += " AND user_id > 0"
	}
	var count struct {
		Total int64 `xorm:"total"`
	}
	query := "SELECT COUNT(*) AS total FROM (SELECT user_id, email FROM repo_contributor_daily WHERE " + where + " GROUP BY user_id, email) temp"
	_, err := db.GetEngine(ctx).SQL(query, args...).Get(&count)
	if err != nil {
		return contributors, 0, err
	}
	return contributors, count.Total, nil
}

func GetContributorActivity(ctx context.Context, repo *repo_model.Repository, timeFrom time.Time, count int) ([]*ContributorSummary, error) {
	if count <= 0 {
		return []*ContributorSummary{}, nil
	}
	start := NewContributorDayStart(timeFrom.UTC())
	rows := make([]*ContributorSummary, 0, count)
	if err := db.GetEngine(ctx).
		Table("repo_contributor_daily").
		Select("user_id, email, author_name, sum(additions) as additions, sum(deletions) as deletions, sum(commits) as commits, sum(changed_files) as changed_files").
		Where("repo_id = ? AND day_start >= ?", repo.ID, start).
		GroupBy("user_id, email, author_name").
		OrderBy("commits DESC").
		Limit(count).
		Find(&rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []*ContributorSummary{}, nil
	}
	return rows, nil
}

type StatsOptions struct {
	Start     *ContributorDayStart
	End       *ContributorDayStart
	UserID    int64
	Email     string
	StatTypes []RepoStatType
}

func repoContributorWeekExpr() string {
	dayNumExpr := "CAST(day_start / 86400000 AS INTEGER)"
	if setting.Database.Type.IsMySQL() {
		dayNumExpr = "day_start DIV 86400000"
	} else if setting.Database.Type.IsMSSQL() {
		dayNumExpr = "day_start / 86400000"
	}
	return fmt.Sprintf("((%s - ((%s + 4) %% 7)) * 86400000)", dayNumExpr, dayNumExpr)
}

// GetRepoWeeklyStats returns aggregated stats per week.
func GetRepoWeeklyStats(ctx context.Context, repoID int64, opts StatsOptions) ([]*WeekData, error) {
	rows := make([]*WeekData, 0, 128)
	if len(opts.StatTypes) == 0 {
		return rows, errors.New("no weekly stat types provided")
	}

	selectParts := make([]string, 0, 4)
	for _, statType := range opts.StatTypes {
		switch statType {
		case RepoStatAdditions:
			selectParts = append(selectParts, "SUM(additions) AS additions")
		case RepoStatDeletions:
			selectParts = append(selectParts, "SUM(deletions) AS deletions")
		case RepoStatCommits:
			selectParts = append(selectParts, "SUM(commits) AS commits")
		case RepoStatChangedFiles:
			selectParts = append(selectParts, "SUM(changed_files) AS changed_files")
		}
	}

	weekExpr := repoContributorWeekExpr()
	query := fmt.Sprintf("SELECT %s AS week, %s FROM repo_contributor_daily WHERE repo_id=?", weekExpr, strings.Join(selectParts, ", "))
	args := []any{repoID}
	if opts.Start != nil {
		query += " AND day_start >= ?"
		args = append(args, int64(*opts.Start))
	}
	if opts.End != nil {
		query += " AND day_start < ?"
		args = append(args, int64(*opts.End))
	}

	if opts.UserID > 0 {
		query += " AND user_id=?"
		args = append(args, opts.UserID)
	} else if opts.Email != "" {
		query += " AND email=?"
		args = append(args, opts.Email)
	}

	query += " GROUP BY week ORDER BY week"
	return rows, db.GetEngine(ctx).SQL(query, args...).Find(&rows)
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
			sess := db.GetEngine(ctx).Incr("additions", update.Additions).
				Incr("deletions", update.Deletions).
				Incr("commits", update.Commits).
				Incr("changed_files", update.ChangedFiles)

			// if it's a registered user, we just use user_id to identify
			if update.UserID > 0 {
				sess = sess.Where("repo_id = ? AND day_start = ? AND user_id = ?", update.RepoID, update.DayStart, update.UserID).
					Cols("updated_unix", "author_name", "email")
			} else { // otherwise, we use email to identify, and user_id is always 0
				sess = sess.Where("repo_id = ? AND day_start = ? AND email = ?", update.RepoID, update.DayStart, update.Email).
					Cols("updated_unix", "author_name")
			}

			updated, err := sess.Update(&ContributorDaily{
				UpdatedUnix: now,
				AuthorName:  update.AuthorName,
				Email:       update.Email,
			})
			if err != nil {
				return err
			}
			if updated > 0 {
				continue
			}

			record := &ContributorDaily{
				RepoID:       update.RepoID,
				DayStart:     update.DayStart,
				UserID:       update.UserID,
				Email:        update.Email,
				AuthorName:   update.AuthorName,
				Additions:    update.Additions,
				Deletions:    update.Deletions,
				Commits:      update.Commits,
				ChangedFiles: update.ChangedFiles,
				UpdatedUnix:  now,
			}
			if _, err := db.GetEngine(ctx).Insert(record); err != nil {
				return err
			}
		}
		return nil
	})
}
