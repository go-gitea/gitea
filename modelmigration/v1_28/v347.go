// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddLFSMirrorPendingAndLFSLastRefsToMirror(_ context.Context, x base.EngineMigration) error {
	type LFSMirrorPending struct {
		ID           int64  `xorm:"pk autoincr"`
		RepositoryID int64  `xorm:"UNIQUE(s) INDEX NOT NULL"`
		Oid          string `xorm:"UNIQUE(s) INDEX NOT NULL"`
		Size         int64  `xorm:"NOT NULL"`
		BlobSha      string `xorm:"NOT NULL DEFAULT ''"`
		CreatedUnix  int64  `xorm:"created"`
	}
	type Mirror struct {
		LFSLastRefs string `xorm:"lfs_last_refs TEXT"`
	}
	if err := x.Sync(new(LFSMirrorPending)); err != nil {
		return err
	}
	return x.Sync(new(Mirror))
}
