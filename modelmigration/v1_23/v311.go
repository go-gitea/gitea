// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23

import (
	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddTimeEstimateColumnToIssueTable(x base.EngineMigration) error {
	type Issue struct {
		TimeEstimate int64 `xorm:"NOT NULL DEFAULT 0"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(Issue))
	return err
}
