// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type improveActionTableIndicesAction struct {
	ID          int64 `xorm:"pk autoincr"`
	UserID      int64 // Receiver user id.
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
func (a *improveActionTableIndicesAction) TableName() string {
	return "action"
}

// TableIndices implements xorm's TableIndices interface
func (a *improveActionTableIndicesAction) TableIndices() []*schemas.Index {
	actUserIndex := schemas.NewIndex("au_r_c_u_d", schemas.IndexType)
	actUserIndex.AddColumn("act_user_id", "repo_id", "created_unix", "user_id", "is_deleted")

	repoIndex := schemas.NewIndex("r_c_u_d", schemas.IndexType)
	repoIndex.AddColumn("repo_id", "created_unix", "user_id", "is_deleted")

	return []*schemas.Index{actUserIndex, repoIndex}
}

func improveActionTableIndices(x *xorm.Engine) error {
	{
		type Action struct {
			ID          int64 `xorm:"pk autoincr"`
			UserID      int64 `xorm:"INDEX"` // Receiver user id.
			OpType      int
			ActUserID   int64 `xorm:"INDEX"` // Action user id.
			RepoID      int64 `xorm:"INDEX"`
			CommentID   int64 `xorm:"INDEX"`
			IsDeleted   bool  `xorm:"INDEX NOT NULL DEFAULT false"`
			RefName     string
			IsPrivate   bool               `xorm:"INDEX NOT NULL DEFAULT false"`
			Content     string             `xorm:"TEXT"`
			CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		}
		if err := x.Sync2(&Action{}); err != nil {
			return err
		}
		if err := x.DropIndexes(&Action{}); err != nil {
			return err
		}
	}
	return x.Sync2(&improveActionTableIndicesAction{})
}
