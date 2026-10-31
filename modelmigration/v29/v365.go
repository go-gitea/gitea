// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddMetadataMirrorSyncColumns(_ context.Context, x base.EngineMigration) error {
	type Comment struct {
		OriginalID int64 `xorm:"index"`
	}
	type Review struct {
		OriginalID int64 `xorm:"index"`
	}
	type Mirror struct {
		SyncIssues       bool `xorm:"NOT NULL DEFAULT false"`
		SyncPullRequests bool `xorm:"NOT NULL DEFAULT false"`
	}
	type PullRequest struct {
		HeadRepoOwner string `xorm:"NOT NULL DEFAULT ''"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(Comment), new(Review), new(Mirror), new(PullRequest))
	return err
}
