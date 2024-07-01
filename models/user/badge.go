// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// Badge represents a user badge
type Badge struct {
	ID          int64  `xorm:"pk autoincr"`
	Slug        string `xorm:"UNIQUE"`
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

func AdminCreateBadge(ctx context.Context, badge *Badge) error {
	return CreateBadge(ctx, badge)
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

// GetBadgeUsers returns the badges users.
func GetBadgeUsers(ctx context.Context, b *Badge) ([]*User, int64, error) {
	sess := db.GetEngine(ctx).
		Select("`user`.*").
		Join("INNER", "user_badge", "`user_badge`.user_id=user.id").
		Where("user_badge.badge_id=?", b.ID)

	users := make([]*User, 0, 8)
	count, err := sess.FindAndCount(&users)
	return users, count, err
}

// CreateBadge creates a new badge.
func CreateBadge(ctx context.Context, badge *Badge) error {
	isExist, err := IsBadgeExist(ctx, 0, badge.Slug)

	if err != nil {
		return err
	} else if isExist {
		return ErrBadgeAlreadyExist{badge.Slug}
	}

	_, err = db.GetEngine(ctx).Insert(badge)

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

// GetBadgeByID returns a badge
func GetBadgeByID(ctx context.Context, id int64) (*Badge, error) {
	badge := new(Badge)
	has, err := db.GetEngine(ctx).Where("id=?", id).Get(badge)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrBadgeNotExist{ID: id}
	}

	return badge, err
}

// UpdateBadge updates a badge based on its slug.
func UpdateBadge(ctx context.Context, badge *Badge) error {
	_, err := db.GetEngine(ctx).Where("id=?", badge.ID).Cols("slug", "description", "image_url").Update(badge)
	return err
}

// DeleteBadge deletes a badge.
func DeleteBadge(ctx context.Context, badge *Badge) error {
	_, err := db.GetEngine(ctx).Where("slug=?", badge.Slug).Delete(badge)
	return err
}

// DeleteUserBadgeRecord deletes a user badge record.
func DeleteUserBadgeRecord(ctx context.Context, badge *Badge) error {
	userBadge := &UserBadge{
		BadgeID: badge.ID,
	}
	_, err := db.GetEngine(ctx).Where("badge_id=?", userBadge.BadgeID).Delete(userBadge)
	return err
}

// AddUserBadge adds a badge to a user.
func AddUserBadge(ctx context.Context, u *User, badge *Badge) error {
	isExist, err := IsBadgeUserExist(ctx, u.ID, badge.ID)
	if err != nil {
		return err
	} else if isExist {
		return ErrBadgeAlreadyExist{}
	}

	return AddUserBadges(ctx, u, []*Badge{badge})
}

// AddUserBadges adds badges to a user.
func AddUserBadges(ctx context.Context, u *User, badges []*Badge) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		for _, badge := range badges {
			// hydrate badge and check if it exists
			has, err := db.GetEngine(ctx).Where("id=?", badge.ID).Get(badge)
			if err != nil {
				return err
			} else if !has {
				return ErrBadgeNotExist{ID: badge.ID}
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
			if _, err := db.GetEngine(ctx).Delete(&UserBadge{BadgeID: badge.ID, UserID: u.ID}); err != nil {
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

// HTMLURL returns the badges full link.
func (u *Badge) HTMLURL() string {
	return setting.AppURL + url.PathEscape(u.Slug)
}

// IsBadgeExist checks if given badge slug exist,
// it is used when creating/updating a badge slug
func IsBadgeExist(ctx context.Context, uid int64, slug string) (bool, error) {
	if len(slug) == 0 {
		return false, nil
	}
	return db.GetEngine(ctx).
		Where("slug!=?", uid).
		Get(&Badge{Slug: strings.ToLower(slug)})
}

// IsBadgeUserExist checks if given badge id, uid exist,
func IsBadgeUserExist(ctx context.Context, uid, bid int64) (bool, error) {
	return db.GetEngine(ctx).
		Get(&UserBadge{UserID: uid, BadgeID: bid})
}

// SearchBadgeOptions represents the options when fdin badges
type SearchBadgeOptions struct {
	db.ListOptions

	Keyword string
	Slug    string
	ID      int64
	OrderBy db.SearchOrderBy
	Actor   *User // The user doing the search

	ExtraParamStrings map[string]string
}

func (opts *SearchBadgeOptions) ToConds() builder.Cond {
	cond := builder.NewCond()

	if opts.Keyword != "" {
		cond = cond.And(builder.Like{"badge.slug", opts.Keyword})
	}

	return cond
}

func (opts *SearchBadgeOptions) ToOrders() string {
	orderBy := "badge.slug"
	return orderBy
}

func (opts *SearchBadgeOptions) ToJoins() []db.JoinFunc {
	return []db.JoinFunc{
		func(e db.Engine) error {
			e.Join("INNER", "badge", "badge.badge_id = badge.id")
			return nil
		},
	}
}

func SearchBadges(ctx context.Context, opts *SearchBadgeOptions) (badges []*Badge, _ int64, _ error) {
	sessCount := opts.toSearchQueryBase(ctx)
	defer sessCount.Close()
	count, err := sessCount.Count(new(Badge))
	if err != nil {
		return nil, 0, fmt.Errorf("count: %w", err)
	}

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = db.SearchOrderByID
	}

	sessQuery := opts.toSearchQueryBase(ctx).OrderBy(opts.OrderBy.String())
	defer sessQuery.Close()
	if opts.Page != 0 {
		sessQuery = db.SetSessionPagination(sessQuery, opts)
	}

	// the sql may contain JOIN, so we must only select Badge related columns
	sessQuery = sessQuery.Select("`badge`.*")
	badges = make([]*Badge, 0, opts.PageSize)
	return badges, count, sessQuery.Find(&badges)
}

func (opts *SearchBadgeOptions) toSearchQueryBase(ctx context.Context) *xorm.Session {
	var cond builder.Cond
	cond = builder.Neq{"id": -1}

	if len(opts.Keyword) > 0 {
		lowerKeyword := strings.ToLower(opts.Keyword)
		keywordCond := builder.Or(
			builder.Like{"slug", lowerKeyword},
			builder.Like{"description", lowerKeyword},
			builder.Like{"id", lowerKeyword},
		)
		cond = cond.And(keywordCond)
	}

	if opts.ID > 0 {
		cond = cond.And(builder.Eq{"id": opts.ID})
	}

	if len(opts.Slug) > 0 {
		cond = cond.And(builder.Eq{"slug": opts.Slug})
	}

	e := db.GetEngine(ctx)

	return e.Where(cond)
}
