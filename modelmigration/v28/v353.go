// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddMergeStateToPullRequest(_ context.Context, x base.EngineMigration) error {
	type PullRequest struct {
		MergeState int `xorm:"NOT NULL DEFAULT 0 INDEX"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(PullRequest))
	return err
}
