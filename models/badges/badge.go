// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package badges

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// Badge represents a reputation badge
type Badge struct {
	ID          int64  `xorm:"pk autoincr"`
	Slug        string `xorm:"UNIQUE"`
	Description string
	ImageURL    string
}

func init() {
	db.RegisterModel(new(Badge))
}

// CreateBadge creates a new badge.
func CreateBadge(ctx context.Context, badge *Badge) error {
	exists, err := db.GetEngine(ctx).Where("slug = ?", badge.Slug).Exist(new(Badge))
	if err != nil {
		return err
	}
	if exists {
		return util.NewAlreadyExistErrorf("badge already exists [slug: %s]", badge.Slug)
	}

	if _, err := db.GetEngine(ctx).Insert(badge); err != nil {
		// Handle race between existence check and insert.
		exists, existErr := db.GetEngine(ctx).Where("slug = ?", badge.Slug).Exist(new(Badge))
		if existErr == nil && exists {
			return util.NewAlreadyExistErrorf("badge already exists [slug: %s]", badge.Slug)
		}
		return err
	}
	return nil
}

// GetBadge returns a specific badge
func GetBadge(ctx context.Context, slug string) (*Badge, error) {
	badge := new(Badge)
	has, err := db.GetEngine(ctx).Where("slug=?", slug).Get(badge)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, util.NewNotExistErrorf("badge does not exist [slug: %s]", slug)
	}
	return badge, nil
}

// UpdateBadge updates a badge based on its slug.
func UpdateBadge(ctx context.Context, badge *Badge) error {
	_, err := db.GetEngine(ctx).Where("slug=?", badge.Slug).Cols("description", "image_url").Update(badge)
	return err
}

// DeleteBadge deletes a badge and all associated entries.
func DeleteBadge(ctx context.Context, badge *Badge) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		// First delete all mapping entries for this badge
		if _, err := db.GetEngine(ctx).Exec("DELETE FROM user_badge WHERE badge_id = ?", badge.ID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Exec("DELETE FROM org_badge WHERE badge_id = ?", badge.ID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Exec("DELETE FROM repo_badge WHERE badge_id = ?", badge.ID); err != nil {
			return err
		}

		// Then delete the badge itself
		if _, err := db.GetEngine(ctx).Where("slug=?", badge.Slug).Delete(badge); err != nil {
			return err
		}
		return nil
	})
}

// SearchBadgeOptions represents the options when finding badges
type SearchBadgeOptions struct {
	db.ListOptions

	Keyword string
	Slug    string
	ID      int64
	OrderBy db.SearchOrderBy
}

func (opts *SearchBadgeOptions) ToConds() builder.Cond {
	cond := builder.NewCond()

	if opts.Keyword != "" {
		keywordCond := builder.Or(
			db.BuildCaseInsensitiveLike("badge.slug", opts.Keyword),
			db.BuildCaseInsensitiveLike("badge.description", opts.Keyword),
		)
		cond = cond.And(keywordCond)
	}

	if opts.ID > 0 {
		cond = cond.And(builder.Eq{"badge.id": opts.ID})
	}

	if len(opts.Slug) > 0 {
		cond = cond.And(builder.Eq{"badge.slug": opts.Slug})
	}

	return cond
}

func (opts *SearchBadgeOptions) ToOrders() string {
	return opts.OrderBy.String()
}

// SearchBadges returns badges based on the provided SearchBadgeOptions options
func SearchBadges(ctx context.Context, opts *SearchBadgeOptions) ([]*Badge, int64, error) {
	return db.FindAndCount[Badge](ctx, opts)
}
