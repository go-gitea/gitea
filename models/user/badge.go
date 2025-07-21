// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
	"xorm.io/xorm/schemas"
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

// TableIndices implements xorm's TableIndices interface
func (n *UserBadge) TableIndices() []*schemas.Index {
	indices := make([]*schemas.Index, 0, 1)
	ubUnique := schemas.NewIndex("unique_user_badge", schemas.UniqueType)
	ubUnique.AddColumn("user_id", "badge_id")
	indices = append(indices, ubUnique)
	return indices
}

// ErrBadgeAlreadyExist represents a "badge already exists" error.
type ErrBadgeAlreadyExist struct {
	Slug string
}

// IsErrBadgeAlreadyExist checks if an error is a ErrBadgeAlreadyExist.
func IsErrBadgeAlreadyExist(err error) bool {
	_, ok := err.(ErrBadgeAlreadyExist)
	return ok
}

func (err ErrBadgeAlreadyExist) Error() string {
	return fmt.Sprintf("badge already exists [slug: %s]", err.Slug)
}

// Unwrap unwraps this error as a ErrExist error
func (err ErrBadgeAlreadyExist) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrBadgeNotExist represents a "BadgeNotExist" kind of error.
type ErrBadgeNotExist struct {
	Slug string
	ID   int64
}

func (err ErrBadgeNotExist) Error() string {
	if err.ID > 0 {
		return fmt.Sprintf("badge does not exist [id: %d]", err.ID)
	}
	return fmt.Sprintf("badge does not exist [slug: %s]", err.Slug)
}

// IsErrBadgeNotExist checks if an error is a ErrBadgeNotExist.
func IsErrBadgeNotExist(err error) bool {
	_, ok := err.(ErrBadgeNotExist)
	return ok
}

// Unwrap unwraps this error as a ErrNotExist error
func (err ErrBadgeNotExist) Unwrap() error {
	return util.ErrNotExist
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

// GetBadgeUsersOptions contains options for getting users with a specific badge
type GetBadgeUsersOptions struct {
	db.ListOptions
	BadgeSlug string
}

// GetBadgeUsers returns the users that have a specific badge with pagination support.
func GetBadgeUsers(ctx context.Context, opts *GetBadgeUsersOptions) ([]*User, int64, error) {
	sess := db.GetEngine(ctx).
		Select("`user`.*").
		Join("INNER", "user_badge", "`user_badge`.user_id=user.id").
		Join("INNER", "badge", "`user_badge`.badge_id=badge.id").
		Where("badge.slug=?", opts.BadgeSlug)

	if opts.Page > 0 {
		sess = db.SetSessionPagination(sess, opts)
	}

	users := make([]*User, 0, opts.PageSize)
	count, err := sess.FindAndCount(&users)
	return users, count, err
}

// CreateBadge creates a new badge.
func CreateBadge(ctx context.Context, badge *Badge) error {
	// this will fail if the badge already exists due to the UNIQUE constraint
	_, err := db.GetEngine(ctx).Insert(badge)
	return err
}

// GetBadge returns a specific badge
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
	_, err := db.GetEngine(ctx).Where("slug=?", badge.Slug).Cols("description", "image_url").Update(badge)
	return err
}

// DeleteBadge deletes a badge and all associated user_badge entries.
func DeleteBadge(ctx context.Context, badge *Badge) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		// First delete all user_badge entries for this badge
		if _, err := db.GetEngine(ctx).
			Where("badge_id = (SELECT id FROM badge WHERE slug = ?)", badge.Slug).
			Delete(&UserBadge{}); err != nil {
			return err
		}

		// Then delete the badge itself
		if _, err := db.GetEngine(ctx).Where("slug=?", badge.Slug).Delete(badge); err != nil {
			return err
		}
		return nil
	})
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
				return ErrBadgeNotExist{Slug: badge.Slug}
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

// RemoveUserBadges removes specific badges from a user.
func RemoveUserBadges(ctx context.Context, u *User, badges []*Badge) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		badgeSlugs := make([]string, 0, len(badges))
		for _, badge := range badges {
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
		if _, err := db.GetEngine(ctx).Table("user_badge").In("id", userBadgeIDs).Delete(); err != nil {
			return err
		}
		return nil
	})
}

// RemoveAllUserBadges removes all badges from a user.
func RemoveAllUserBadges(ctx context.Context, u *User) error {
	_, err := db.GetEngine(ctx).Where("user_id=?", u.ID).Delete(&UserBadge{})
	return err
}

// SearchBadgeOptions represents the options when fdin badges
type SearchBadgeOptions struct {
	db.ListOptions

	Keyword string
	Slug    string
	ID      int64
	OrderBy db.SearchOrderBy
	Actor   *User // The user doing the search
}

func (opts *SearchBadgeOptions) ToConds() builder.Cond {
	cond := builder.NewCond()

	if opts.Keyword != "" {
		lowerKeyword := strings.ToLower(opts.Keyword)
		keywordCond := builder.Or(
			builder.Like{"badge.slug", lowerKeyword},
			builder.Like{"badge.description", lowerKeyword},
			builder.Like{"badge.id", lowerKeyword},
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

// SearchBadges returns badges based on the provided SearchBadgeOptions options
func SearchBadges(ctx context.Context, opts *SearchBadgeOptions) ([]*Badge, int64, error) {
	return db.FindAndCount[Badge](ctx, opts)
}

// GetBadgeByID returns a specific badge by ID
func GetBadgeByID(ctx context.Context, id int64) (*Badge, error) {
	badge := new(Badge)
	has, err := db.GetEngine(ctx).ID(id).Get(badge)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrBadgeNotExist{ID: id}
	}
	return badge, nil
}
