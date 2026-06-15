// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

type commentWithRepoID struct {
	RepoID int64 `xorm:"INDEX"`
}

func (commentWithRepoID) TableName() string {
	return "comment"
}

// AddRepoIDToComment adds a repo_id column to the comment table. It is required
// for comments that are not bound to an issue (e.g. inline comments on commits)
// so they can be scoped to a repository without leaking across forks that share
// the same commit SHA.
func AddRepoIDToComment(x db.EngineMigration) error {
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(commentWithRepoID))
	return err
}
