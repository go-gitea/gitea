// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
)

// Badge represents a user badge
type Badge struct {
	ID          int64  `xorm:"pk autoincr"`
	Slug        string `xorm:"UNIQUE"`
	Description string
	ImageURL    string
}

// UserBadge represents a user badge
type UserBadge struct { //nolint:revive // export stutter
	ID      int64 `xorm:"pk autoincr"`
	BadgeID int64
	UserID  int64 `xorm:"INDEX"`
}

func init() {
	db.RegisterModel(new(Badge))
	db.RegisterModel(new(UserBadge))
}

// GetUserBadges returns the user's badges.
func GetUserBadges(ctx context.Context, u *User) ([]*Badge, int64, error) {
	sess := db.GetEngine(ctx).
		Select("`badge`.*").
		Join("INNER", "user_badge", "`user_badge`.badge_id=badge.id").
		Where("user_badge.user_id=?", u.ID)

	badges := make([]*Badge, 0, 8)
	count, err := sess.FindAndCount(&badges)
	return badges, count, err
}

// CreateBadge creates a new badge.
func CreateBadge(ctx context.Context, badge *Badge) error {
	_, err := db.GetEngine(ctx).Insert(badge)
	return err
}

// GetBadge returns a badge
func GetBadge(ctx context.Context, slug string) (*Badge, error) {
	badge := new(Badge)
	has, err := db.GetEngine(ctx).Where("slug=?", slug).Get(badge)
	if !has {
		return nil, err
	}
	return badge, err
}

// UpdateBadge updates a badge based on its slug.
func UpdateBadge(ctx context.Context, badge *Badge) error {
	_, err := db.GetEngine(ctx).Where("slug=?", badge.Slug).Update(badge)
	return err
}

// DeleteBadge deletes a badge.
func DeleteBadge(ctx context.Context, badge *Badge) error {
	_, err := db.GetEngine(ctx).Where("slug=?", badge.Slug).Delete(badge)
	return err
}

// AddUserBadge adds a badge to a user.
func AddUserBadge(ctx context.Context, u *User, badge *Badge) error {
	return AddUserBadges(ctx, u, []*Badge{badge})
}

// AddUserBadges adds badges to a user.
func AddUserBadges(ctx context.Context, u *User, badges []*Badge) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		for _, badge := range badges {
			// hydrate badge and check if it exists
			has, err := db.GetEngine(ctx).Where("slug=?", badge.Slug).Get(badge)
			if err != nil {
				return err
			} else if !has {
				return fmt.Errorf("badge with slug %s doesn't exist", badge.Slug)
			}
			if err := db.Insert(ctx, &UserBadge{
				BadgeID: badge.ID,
				UserID:  u.ID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// RemoveUserBadge removes a badge from a user.
func RemoveUserBadge(ctx context.Context, u *User, badge *Badge) error {
	return RemoveUserBadges(ctx, u, []*Badge{badge})
}

// RemoveUserBadges removes badges from a user.
func RemoveUserBadges(ctx context.Context, u *User, badges []*Badge) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		for _, badge := range badges {
			if _, err := db.GetEngine(ctx).
				Join("INNER", "badge", "badge.id = `user_badge`.badge_id").
				Where("`user_badge`.user_id=? AND `badge`.slug=?", u.ID, badge.Slug).
				Delete(&UserBadge{}); err != nil {
				return err
			}
		}
		return nil
	})
}

// RemoveAllUserBadges removes all badges from a user.
func RemoveAllUserBadges(ctx context.Context, u *User) error {
	_, err := db.GetEngine(ctx).Where("user_id=?", u.ID).Delete(&UserBadge{})
	return err
}
