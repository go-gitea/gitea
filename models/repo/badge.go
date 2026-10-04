// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"gitea.dev/models/badges"
	"gitea.dev/models/db"
	"gitea.dev/modules/util"

	"xorm.io/xorm/schemas"
)

// RepoBadge represents a repository badge
type RepoBadge struct { //nolint:revive // export stutter
	ID      int64 `xorm:"pk autoincr"`
	BadgeID int64
	RepoID  int64
}

// TableIndices implements xorm's TableIndices interface
func (n *RepoBadge) TableIndices() []*schemas.Index {
	indices := make([]*schemas.Index, 0, 1)
	ubUnique := schemas.NewIndex("unique_repo_badge", schemas.UniqueType)
	ubUnique.AddColumn("repo_id", "badge_id")
	indices = append(indices, ubUnique)
	return indices
}

func init() {
	db.RegisterModel(new(RepoBadge))
}

// GetRepoBadges returns the repo's badges.
func GetRepoBadges(ctx context.Context, repo *Repository) ([]*badges.Badge, int64, error) {
	sess := db.GetEngine(ctx).
		Select("`badge`.*").
		Join("INNER", "repo_badge", "`repo_badge`.badge_id=badge.id").
		Where("repo_badge.repo_id=?", repo.ID)

	badgesSlice := make([]*badges.Badge, 0, 8)
	count, err := sess.FindAndCount(&badgesSlice)
	return badgesSlice, count, err
}

// AddRepoBadge adds a badge to a repository.
func AddRepoBadge(ctx context.Context, repo *Repository, badge *badges.Badge) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		// hydrate badge and check if it exists
		has, err := db.GetEngine(ctx).Where("slug=?", badge.Slug).Get(badge)
		if err != nil {
			return err
		} else if !has {
			return util.NewNotExistErrorf("badge does not exist [slug: %s]", badge.Slug)
		}

		exists, err := db.GetEngine(ctx).Where("badge_id = ? AND repo_id = ?", badge.ID, repo.ID).Exist(new(RepoBadge))
		if err != nil {
			return err
		}
		if exists {
			return util.NewAlreadyExistErrorf("repo badge already exists [repo_id: %d, badge_id: %d]", repo.ID, badge.ID)
		}

		if err := db.Insert(ctx, &RepoBadge{
			BadgeID: badge.ID,
			RepoID:  repo.ID,
		}); err != nil {
			exists, existErr := db.GetEngine(ctx).Where("badge_id = ? AND repo_id = ?", badge.ID, repo.ID).Exist(new(RepoBadge))
			if existErr == nil && exists {
				return util.NewAlreadyExistErrorf("repo badge already exists [repo_id: %d, badge_id: %d]", repo.ID, badge.ID)
			}
			return err
		}
		return nil
	})
}

// RemoveRepoBadge removes a badge from a repository.
func RemoveRepoBadge(ctx context.Context, repo *Repository, badge *badges.Badge) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		var userBadges []RepoBadge
		if err := db.GetEngine(ctx).Table("repo_badge").
			Join("INNER", "badge", "badge.id = `repo_badge`.badge_id").
			Where("`repo_badge`.repo_id = ?", repo.ID).In("`badge`.slug", []string{badge.Slug}).
			Find(&userBadges); err != nil {
			return err
		}
		userBadgeIDs := make([]int64, 0, len(userBadges))
		for _, ub := range userBadges {
			userBadgeIDs = append(userBadgeIDs, ub.ID)
		}
		if len(userBadgeIDs) == 0 {
			return nil
		}
		if _, err := db.GetEngine(ctx).Table("repo_badge").In("id", userBadgeIDs).Delete(); err != nil {
			return err
		}
		return nil
	})
}

// GetBadgeReposOptions contains options for getting repos with a specific badge
type GetBadgeReposOptions struct {
	db.ListOptions
	BadgeSlug string
}

// GetBadgeRepos returns the repos that have a specific badge with pagination support.
func GetBadgeRepos(ctx context.Context, opts *GetBadgeReposOptions) ([]*Repository, int64, error) {
	sess := db.GetEngine(ctx).
		Select("`repository`.*").
		Join("INNER", "repo_badge", "`repo_badge`.repo_id=`repository`.id").
		Join("INNER", "badge", "`repo_badge`.badge_id=badge.id").
		Where("badge.slug=?", opts.BadgeSlug)

	if opts.Page > 0 {
		db.SetSessionPagination(sess, opts)
	}

	repos := make([]*Repository, 0, opts.PageSize)
	count, err := sess.FindAndCount(&repos)
	return repos, count, err
}
