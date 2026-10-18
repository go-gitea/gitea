// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"slices"

	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// RepoContributor holds an author's default branch commits including merges, keyed by UserID if matched to a user, else by lower-cased Email
type RepoContributor struct { //revive:disable-line:exported
	ID        int64  `xorm:"pk autoincr"`
	RepoID    int64  `xorm:"UNIQUE(s) NOT NULL"`
	UserID    int64  `xorm:"UNIQUE(s) NOT NULL DEFAULT 0"`
	Email     string `xorm:"UNIQUE(s) VARCHAR(255) NOT NULL DEFAULT ''"`
	Name      string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	Commits   int64  `xorm:"NOT NULL DEFAULT 0"`
	Additions int64  `xorm:"NOT NULL DEFAULT 0"`
	Deletions int64  `xorm:"NOT NULL DEFAULT 0"`

	User *user_model.User `xorm:"-"`
}

// RepoContributorWeek holds the non-merge commits of a RepoContributor per week, Week being the Unix milliseconds of its starting Sunday
type RepoContributorWeek struct { //revive:disable-line:exported
	ID        int64  `xorm:"pk autoincr"`
	RepoID    int64  `xorm:"UNIQUE(s) NOT NULL"`
	UserID    int64  `xorm:"UNIQUE(s) NOT NULL DEFAULT 0"`
	Email     string `xorm:"UNIQUE(s) VARCHAR(255) NOT NULL DEFAULT ''"`
	Week      int64  `xorm:"UNIQUE(s) NOT NULL"`
	Commits   int64  `xorm:"NOT NULL DEFAULT 0"`
	Additions int64  `xorm:"NOT NULL DEFAULT 0"`
	Deletions int64  `xorm:"NOT NULL DEFAULT 0"`
}

func init() {
	db.RegisterModel(new(RepoContributor))
	db.RegisterModel(new(RepoContributorWeek))
}

type FindRepoContributorsOptions struct {
	db.ListOptions
	RepoID    int64
	OnlyUsers bool
}

func (opts FindRepoContributorsOptions) ToConds() builder.Cond {
	cond := builder.NewCond().And(builder.Eq{"repo_id": opts.RepoID})
	if opts.OnlyUsers {
		cond = cond.And(builder.Gt{"user_id": 0})
	}
	return cond
}

func (opts FindRepoContributorsOptions) ToOrders() string {
	return "commits DESC, id ASC"
}

func GetRepoContributorWeeks(ctx context.Context, repoID int64) ([]*RepoContributorWeek, error) {
	weeks := make([]*RepoContributorWeek, 0, 64)
	return weeks, db.GetEngine(ctx).Where("repo_id = ?", repoID).Find(&weeks)
}

func GetRepoWeeklyTotals(ctx context.Context, repoID int64) ([]*RepoContributorWeek, error) {
	weeks := make([]*RepoContributorWeek, 0, 64)
	return weeks, db.GetEngine(ctx).Table("repo_contributor_week").
		Select("week, SUM(commits) AS commits, SUM(additions) AS additions, SUM(deletions) AS deletions").
		Where("repo_id = ?", repoID).GroupBy("week").Find(&weeks)
}

func ReplaceRepoContributors(ctx context.Context, repoID int64, contributors []*RepoContributor, weeks []*RepoContributorWeek) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := db.DeleteBeans(ctx, &RepoContributor{RepoID: repoID}, &RepoContributorWeek{RepoID: repoID}); err != nil {
			return err
		}
		if err := insertInBatches(ctx, contributors); err != nil {
			return err
		}
		return insertInBatches(ctx, weeks)
	})
}

func insertInBatches[T any](ctx context.Context, beans []*T) error {
	for batch := range slices.Chunk(beans, db.MaxBatchInsertSize(new(T))) {
		if _, err := db.GetEngine(ctx).Insert(batch); err != nil {
			return err
		}
	}
	return nil
}

func AddRepoContributors(ctx context.Context, contributors []*RepoContributor, weeks []*RepoContributorWeek) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		for _, c := range contributors {
			sess := db.GetEngine(ctx).Where("repo_id = ? AND user_id = ? AND email = ?", c.RepoID, c.UserID, c.Email).
				Incr("commits", c.Commits).Incr("additions", c.Additions).Incr("deletions", c.Deletions)
			if err := updateOrInsert(ctx, sess, &RepoContributor{Name: c.Name}, c); err != nil {
				return err
			}
		}
		for _, w := range weeks {
			sess := db.GetEngine(ctx).Where("repo_id = ? AND user_id = ? AND email = ? AND week = ?", w.RepoID, w.UserID, w.Email, w.Week).
				Incr("commits", w.Commits).Incr("additions", w.Additions).Incr("deletions", w.Deletions)
			if err := updateOrInsert(ctx, sess, &RepoContributorWeek{}, w); err != nil {
				return err
			}
		}
		return nil
	})
}

func updateOrInsert(ctx context.Context, sess *xorm.Session, update, insert any) error {
	updated, err := sess.Update(update)
	if err != nil || updated > 0 {
		return err
	}
	return db.Insert(ctx, insert)
}
