// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"

	"gitea.dev/models/badges"
	"gitea.dev/models/db"
	"gitea.dev/modules/util"

	"xorm.io/xorm/schemas"
)

// OrgBadge represents an organization badge
type OrgBadge struct {
	ID      int64 `xorm:"pk autoincr"`
	BadgeID int64
	OrgID   int64
}

// TableIndices implements xorm's TableIndices interface
func (n *OrgBadge) TableIndices() []*schemas.Index {
	indices := make([]*schemas.Index, 0, 1)
	ubUnique := schemas.NewIndex("unique_org_badge", schemas.UniqueType)
	ubUnique.AddColumn("org_id", "badge_id")
	indices = append(indices, ubUnique)
	return indices
}

func init() {
	db.RegisterModel(new(OrgBadge))
}

// GetOrgBadges returns the org's badges.
func GetOrgBadges(ctx context.Context, org *Organization) ([]*badges.Badge, int64, error) {
	sess := db.GetEngine(ctx).
		Select("`badge`.*").
		Join("INNER", "org_badge", "`org_badge`.badge_id=badge.id").
		Where("org_badge.org_id=?", org.ID)

	badgesSlice := make([]*badges.Badge, 0, 8)
	count, err := sess.FindAndCount(&badgesSlice)
	return badgesSlice, count, err
}

// AddOrgBadge adds a badge to an organization.
func AddOrgBadge(ctx context.Context, org *Organization, badge *badges.Badge) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		// hydrate badge and check if it exists
		has, err := db.GetEngine(ctx).Where("slug=?", badge.Slug).Get(badge)
		if err != nil {
			return err
		} else if !has {
			return util.NewNotExistErrorf("badge does not exist [slug: %s]", badge.Slug)
		}

		exists, err := db.GetEngine(ctx).Where("badge_id = ? AND org_id = ?", badge.ID, org.ID).Exist(new(OrgBadge))
		if err != nil {
			return err
		}
		if exists {
			return util.NewAlreadyExistErrorf("org badge already exists [org_id: %d, badge_id: %d]", org.ID, badge.ID)
		}

		if err := db.Insert(ctx, &OrgBadge{
			BadgeID: badge.ID,
			OrgID:   org.ID,
		}); err != nil {
			exists, existErr := db.GetEngine(ctx).Where("badge_id = ? AND org_id = ?", badge.ID, org.ID).Exist(new(OrgBadge))
			if existErr == nil && exists {
				return util.NewAlreadyExistErrorf("org badge already exists [org_id: %d, badge_id: %d]", org.ID, badge.ID)
			}
			return err
		}
		return nil
	})
}

// RemoveOrgBadge removes a badge from an organization.
func RemoveOrgBadge(ctx context.Context, org *Organization, badge *badges.Badge) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		var userBadges []OrgBadge
		if err := db.GetEngine(ctx).Table("org_badge").
			Join("INNER", "badge", "badge.id = `org_badge`.badge_id").
			Where("`org_badge`.org_id = ?", org.ID).In("`badge`.slug", []string{badge.Slug}).
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
		if _, err := db.GetEngine(ctx).Table("org_badge").In("id", userBadgeIDs).Delete(); err != nil {
			return err
		}
		return nil
	})
}

// GetBadgeOrgsOptions contains options for getting orgs with a specific badge
type GetBadgeOrgsOptions struct {
	db.ListOptions
	BadgeSlug string
}

// GetBadgeOrgs returns the orgs that have a specific badge with pagination support.
func GetBadgeOrgs(ctx context.Context, opts *GetBadgeOrgsOptions) ([]*Organization, int64, error) {
	sess := db.GetEngine(ctx).
		Select("`user`.*").
		Join("INNER", "org_badge", "`org_badge`.org_id=`user`.id").
		Join("INNER", "badge", "`org_badge`.badge_id=badge.id").
		Where("badge.slug=?", opts.BadgeSlug)

	if opts.Page > 0 {
		db.SetSessionPagination(sess, opts)
	}

	orgs := make([]*Organization, 0, opts.PageSize)
	count, err := sess.FindAndCount(&orgs)
	return orgs, count, err
}
