// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"

	"gitea.dev/models/badges"
	"gitea.dev/models/db"
	"gitea.dev/modules/util"

	"xorm.io/xorm/schemas"
)

// UserBadge represents a user badge link
type UserBadge struct { //nolint:revive // export stutter
	ID      int64 `xorm:"pk autoincr"`
	BadgeID int64
	UserID  int64
}

// TableIndices implements xorm's TableIndices interface
func (n *UserBadge) TableIndices() []*schemas.Index {
	indices := make([]*schemas.Index, 0, 1)
	ubUnique := schemas.NewIndex("unique_user_badge", schemas.UniqueType)
	ubUnique.AddColumn("user_id", "badge_id")
	indices = append(indices, ubUnique)
	return indices
}

func init() {
	db.RegisterModel(new(UserBadge))
}

// GetUserBadges returns the user's badges.
func GetUserBadges(ctx context.Context, u *User) ([]*badges.Badge, int64, error) {
	sess := db.GetEngine(ctx).
		Select("`badge`.*").
		Join("INNER", "user_badge", "`user_badge`.badge_id=badge.id").
		Where("user_badge.user_id=?", u.ID)

	b := make([]*badges.Badge, 0, 8)
	count, err := sess.FindAndCount(&b)
	return b, count, err
}

// GetBadgeUsersOptions contains options for getting users with a specific badge
type GetBadgeUsersOptions struct {
	db.ListOptions
	BadgeSlug string
}

// GetBadgeUsers returns the users that have a specific badge with pagination support.
func GetBadgeUsers(ctx context.Context, opts *GetBadgeUsersOptions) ([]*User, int64, error) {
	sess := db.GetEngine(ctx).
		Select("`user`.*").
		Join("INNER", "user_badge", "`user_badge`.user_id=`user`.id").
		Join("INNER", "badge", "`user_badge`.badge_id=badge.id").
		Where("badge.slug=?", opts.BadgeSlug)

	if opts.Page > 0 {
		db.SetSessionPagination(sess, opts)
	}

	users := make([]*User, 0, opts.PageSize)
	count, err := sess.FindAndCount(&users)
	return users, count, err
}

// AddUserBadge adds a badge to a user.
func AddUserBadge(ctx context.Context, u *User, badge *badges.Badge) error {
	return AddUserBadges(ctx, u, []*badges.Badge{badge})
}

// AddUserBadges adds badges to a user.
func AddUserBadges(ctx context.Context, u *User, bdgs []*badges.Badge) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		for _, badge := range bdgs {
			// hydrate badge and check if it exists
			has, err := db.GetEngine(ctx).Where("slug=?", badge.Slug).Get(badge)
			if err != nil {
				return err
			} else if !has {
				return util.NewNotExistErrorf("badge does not exist [slug: %s]", badge.Slug)
			}

			exists, err := db.GetEngine(ctx).Where("badge_id = ? AND user_id = ?", badge.ID, u.ID).Exist(new(UserBadge))
			if err != nil {
				return err
			}
			if exists {
				return util.NewAlreadyExistErrorf("user badge already exists [user_id: %d, badge_id: %d]", u.ID, badge.ID)
			}

			if err := db.Insert(ctx, &UserBadge{
				BadgeID: badge.ID,
				UserID:  u.ID,
			}); err != nil {
				exists, existErr := db.GetEngine(ctx).Where("badge_id = ? AND user_id = ?", badge.ID, u.ID).Exist(new(UserBadge))
				if existErr == nil && exists {
					return util.NewAlreadyExistErrorf("user badge already exists [user_id: %d, badge_id: %d]", u.ID, badge.ID)
				}
				return err
			}
		}
		return nil
	})
}

// RemoveUserBadge removes a badge from a user.
func RemoveUserBadge(ctx context.Context, u *User, badge *badges.Badge) error {
	return RemoveUserBadges(ctx, u, []*badges.Badge{badge})
}

// RemoveUserBadges removes specific badges from a user.
func RemoveUserBadges(ctx context.Context, u *User, bdgs []*badges.Badge) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if len(bdgs) == 0 {
			return nil
		}

		badgeSlugs := make([]string, 0, len(bdgs))
		for _, badge := range bdgs {
			badgeSlugs = append(badgeSlugs, badge.Slug)
		}
		var userBadges []UserBadge
		if err := db.GetEngine(ctx).Table("user_badge").
			Join("INNER", "badge", "badge.id = `user_badge`.badge_id").
			Where("`user_badge`.user_id = ?", u.ID).In("`badge`.slug", badgeSlugs).
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
		if _, err := db.GetEngine(ctx).Table("user_badge").In("id", userBadgeIDs).Delete(); err != nil {
			return err
		}
		return nil
	})
}
