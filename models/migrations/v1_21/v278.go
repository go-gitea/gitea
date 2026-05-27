// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import "gitea.dev/models/db"

func AddIndexToCommentDependentIssueID(x db.EngineMigration) error {
	type Comment struct {
		DependentIssueID int64 `xorm:"index"`
	}

	return x.Sync(new(Comment))
}
