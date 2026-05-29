// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import (
	"fmt"

	"gitea.dev/models/db"
)

func AddChangedProtectedFilesPullRequestColumn(x db.EngineMigration) error {
	type PullRequest struct {
		ChangedProtectedFiles []string `xorm:"TEXT JSON"`
	}

	if err := x.Sync(new(PullRequest)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
