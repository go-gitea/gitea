// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddImmutableReleases(_ context.Context, x base.EngineMigration) error {
	type Release struct {
		IsImmutable bool `xorm:"NOT NULL DEFAULT false"`
	}

	type ImmutableTag struct {
		ID             int64  `xorm:"pk autoincr"`
		LowerOwnerName string `xorm:"UNIQUE(s) NOT NULL"`
		LowerRepoName  string `xorm:"UNIQUE(s) NOT NULL"`
		TagName        string `xorm:"UNIQUE(s) NOT NULL"`
	}

	if err := x.Sync(new(ImmutableTag)); err != nil {
		return err
	}

	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(Release)); err != nil {
		return err
	}

	// tag names are matched exactly now, so a fresh install no longer has this column
	sess := x.NewSession()
	defer sess.Close()
	return base.DropTableColumns(sess, "release", "lower_tag_name")
}
