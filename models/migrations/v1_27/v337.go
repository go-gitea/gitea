// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm/schemas"
)

// actionWithUpdatedIndex is a minimal mirror of the action table used to apply
// the updated c_u composite index (user_id, is_deleted, created_unix).
// The previous index only covered (user_id, is_deleted), which forced the
// database to sort all matching rows by created_unix before returning a page,
// causing multi-second query times on large action tables.
type actionWithUpdatedIndex struct {
	ID          int64              `xorm:"pk autoincr"`
	UserID      int64              `xorm:"INDEX"`
	OpType      int
	ActUserID   int64
	RepoID      int64
	CommentID   int64              `xorm:"INDEX"`
	IsDeleted   bool               `xorm:"NOT NULL DEFAULT false"`
	RefName     string
	IsPrivate   bool               `xorm:"NOT NULL DEFAULT false"`
	Content     string             `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

func (actionWithUpdatedIndex) TableName() string { return "action" }

func (actionWithUpdatedIndex) TableIndices() []*schemas.Index {
	repoIndex := schemas.NewIndex("r_u_d", schemas.IndexType)
	repoIndex.AddColumn("repo_id", "user_id", "is_deleted")

	actUserIndex := schemas.NewIndex("au_r_c_u_d", schemas.IndexType)
	actUserIndex.AddColumn("act_user_id", "repo_id", "created_unix", "user_id", "is_deleted")

	cudIndex := schemas.NewIndex("c_u_d", schemas.IndexType)
	cudIndex.AddColumn("created_unix", "user_id", "is_deleted")

	// Extended from (user_id, is_deleted) to include created_unix so that
	// ORDER BY created_unix DESC on the dashboard query is satisfied by the
	// index without a full sort of all matching rows.
	cuIndex := schemas.NewIndex("c_u", schemas.IndexType)
	cuIndex.AddColumn("user_id", "is_deleted", "created_unix")

	actUserUserIndex := schemas.NewIndex("au_c_u", schemas.IndexType)
	actUserUserIndex.AddColumn("act_user_id", "created_unix", "user_id")

	return []*schemas.Index{actUserIndex, repoIndex, cudIndex, cuIndex, actUserUserIndex}
}

// AddCreatedUnixToActionUserIsDeletedIndex extends the c_u composite index on
// the action table to include created_unix, enabling efficient ORDER BY on the
// dashboard feed query without a full sort of all matching rows.
func AddCreatedUnixToActionUserIsDeletedIndex(x db.EngineMigration) error {
	return x.Sync(new(actionWithUpdatedIndex))
}
