// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddTeamReviewRequestSupport(_ context.Context, x base.EngineMigration) error {
	type Review struct {
		ReviewerTeamID int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	type Comment struct {
		AssigneeTeamID int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	if err := x.Sync(new(Review)); err != nil {
		return err
	}

	return x.Sync(new(Comment))
}
