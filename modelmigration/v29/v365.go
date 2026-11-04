// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"
)

func AddStorageQuotas(_ context.Context, x base.EngineMigration) error {
	type QuotaPolicy struct {
		ID         int64  `xorm:"pk autoincr"`
		Subject    string `xorm:"VARCHAR(20) UNIQUE(subject) NOT NULL"`
		SubjectID  int64  `xorm:"UNIQUE(subject) NOT NULL"`
		GitLimit   *int64
		LFSLimit   *int64
		TotalLimit *int64
	}
	type QuotaUsage struct {
		RepoID      int64              `xorm:"pk"`
		OwnerID     int64              `xorm:"INDEX NOT NULL"`
		GitBytes    int64              `xorm:"NOT NULL"`
		LFSBytes    int64              `xorm:"NOT NULL"`
		Updated     timeutil.TimeStamp `xorm:"updated"`
		GCRequested timeutil.TimeStamp
	}
	type QuotaOperation struct {
		OwnerID       int64              `xorm:"pk"`
		RepoID        int64              `xorm:"INDEX NOT NULL"`
		Token         string             `xorm:"VARCHAR(64) UNIQUE NOT NULL"`
		GitReserved   int64              `xorm:"NOT NULL"`
		LFSReserved   int64              `xorm:"NOT NULL"`
		Created       timeutil.TimeStamp `xorm:"created"`
		NeedsRecovery bool               `xorm:"NOT NULL DEFAULT false"`
		Children      int64              `xorm:"NOT NULL DEFAULT 0"`
		Closing       bool               `xorm:"NOT NULL DEFAULT false"`
	}
	return x.Sync(new(QuotaPolicy), new(QuotaUsage), new(QuotaOperation))
}
