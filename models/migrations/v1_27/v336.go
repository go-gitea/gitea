// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

type pullAutoMergeWithErrorMessage struct {
	ErrorMessage string `xorm:"LONGTEXT"`
}

func (pullAutoMergeWithErrorMessage) TableName() string {
	return "pull_auto_merge"
}

// AddErrorMessageToAutoMerge adds storage for the latest failed auto merge error.
func AddErrorMessageToAutoMerge(x db.EngineMigration) error {
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(pullAutoMergeWithErrorMessage))
	return err
}
