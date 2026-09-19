// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddIndexToIssueUserIssueID(_ context.Context, x base.EngineMigration) error {
	type IssueUser struct {
		IssueID int64 `xorm:"INDEX"`
	}

	return x.Sync(new(IssueUser))
}
