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
		ID            int64  `xorm:"pk autoincr"`
		RepoID        int64  `xorm:"UNIQUE(r) NOT NULL"`
		OwnerID       int64  `xorm:"INDEX(s) NOT NULL"`
		LowerRepoName string `xorm:"INDEX(s) NOT NULL"`
		LowerTagName  string `xorm:"INDEX(s) UNIQUE(r) NOT NULL"`
	}

	if err := x.Sync(new(ImmutableTag)); err != nil {
		return err
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(Release))
	return err
}
