// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"

	"code.gitea.io/gitea/models/db"
)

// Badge represents a user badge
type Badge struct {
	ID          int64 `xorm:"pk autoincr"`
	Description string
	ImageURL    string
}

// UserBadge represents a user badge
type UserBadge struct { //nolint:revive
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
