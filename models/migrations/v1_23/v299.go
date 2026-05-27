// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

func AddContentVersionToIssueAndComment(x db.EngineMigration) error {
	type Issue struct {
		ContentVersion int `xorm:"NOT NULL DEFAULT 0"`
	}

	type Comment struct {
		ContentVersion int `xorm:"NOT NULL DEFAULT 0"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(Comment), new(Issue))
	return err
}
