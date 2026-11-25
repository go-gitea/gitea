// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type OrgBadge struct {
	ID      int64 `xorm:"pk autoincr"`
	BadgeID int64
	OrgID   int64
}

func (b *OrgBadge) TableIndices() []*schemas.Index {
	index := schemas.NewIndex("unique_org_badge", schemas.UniqueType)
	index.AddColumn("org_id", "badge_id")
	return []*schemas.Index{index}
}

type RepoBadge struct {
	ID      int64 `xorm:"pk autoincr"`
	BadgeID int64
	RepoID  int64
}

func (b *RepoBadge) TableIndices() []*schemas.Index {
	index := schemas.NewIndex("unique_repo_badge", schemas.UniqueType)
	index.AddColumn("repo_id", "badge_id")
	return []*schemas.Index{index}
}

// AddRepositoryAndOrganizationBadges creates mappings for reputation labels.
func AddRepositoryAndOrganizationBadges(_ context.Context, x base.EngineMigration) error {
	return x.Sync(new(OrgBadge), new(RepoBadge))
}

// AddColorToBadges adds the display color for manually managed labels.
func AddColorToBadges(_ context.Context, x base.EngineMigration) error {
	type Badge struct {
		Color string
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(Badge))
	return err
}
