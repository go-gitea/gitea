// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import (
	"context"
	"fmt"

	"gitea.dev/modelmigration/base"
)

func AddChangedProtectedFilesPullRequestColumn(_ context.Context, x base.EngineMigration) error {
	type PullRequest struct {
		ChangedProtectedFiles []string `xorm:"TEXT JSON"`
	}

	if err := x.Sync(new(PullRequest)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
