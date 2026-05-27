// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import "gitea.dev/models/db"

func AddIndexToIssueUserIssueID(x db.EngineMigration) error {
	type IssueUser struct {
		IssueID int64 `xorm:"INDEX"`
	}

	return x.Sync(new(IssueUser))
}
