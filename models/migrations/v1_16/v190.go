// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"fmt"

	"gitea.dev/models/db"
)

func AddAgitFlowPullRequest(x db.EngineMigration) error {
	type PullRequestFlow int

	type PullRequest struct {
		Flow PullRequestFlow `xorm:"NOT NULL DEFAULT 0"`
	}

	if err := x.Sync(new(PullRequest)); err != nil {
		return fmt.Errorf("sync2: %w", err)
	}
	return nil
}
