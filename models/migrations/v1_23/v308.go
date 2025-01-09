// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type improveActionTableIndicesAction struct {
	ID          int64 `xorm:"pk autoincr"`
	UserID      int64 `xorm:"INDEX"` // Receiver user id.
	OpType      int
	ActUserID   int64 // Action user id.
	RepoID      int64
	CommentID   int64 `xorm:"INDEX"`
	IsDeleted   bool  `xorm:"NOT NULL DEFAULT false"`
	RefName     string
	IsPrivate   bool               `xorm:"NOT NULL DEFAULT false"`
	Content     string             `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

// TableName sets the name of this table
func (*improveActionTableIndicesAction) TableName() string {
	return "action"
}

func (a *improveActionTableIndicesAction) TableIndices() []*schemas.Index {
	repoIndex := schemas.NewIndex("r_u_d", schemas.IndexType)
	repoIndex.AddColumn("repo_id", "user_id", "is_deleted")

	actUserIndex := schemas.NewIndex("au_r_c_u_d", schemas.IndexType)
	actUserIndex.AddColumn("act_user_id", "repo_id", "created_unix", "user_id", "is_deleted")

	cudIndex := schemas.NewIndex("c_u_d", schemas.IndexType)
	cudIndex.AddColumn("created_unix", "user_id", "is_deleted")

	cuIndex := schemas.NewIndex("c_u", schemas.IndexType)
	cuIndex.AddColumn("user_id", "is_deleted")

	indices := []*schemas.Index{actUserIndex, repoIndex, cudIndex, cuIndex}

	return indices
}

func AddNewIndexForUserDashboard(x *xorm.Engine) error {
	return x.Sync(new(improveActionTableIndicesAction))
}
