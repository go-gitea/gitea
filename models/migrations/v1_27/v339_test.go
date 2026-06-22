// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"context"
	"testing"

	"gitea.dev/models/migrations/migrationtest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

type actionBeforeV339 struct {
	ID          int64 `xorm:"pk autoincr"`
	UserID      int64 `xorm:"INDEX"`
	OpType      int
	ActUserID   int64
	RepoID      int64
	CommentID   int64 `xorm:"INDEX"`
	IsDeleted   bool  `xorm:"NOT NULL DEFAULT false"`
	RefName     string
	IsPrivate   bool               `xorm:"NOT NULL DEFAULT false"`
	Content     string             `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

func (actionBeforeV339) TableName() string { return "action" }

func (actionBeforeV339) TableIndices() []*schemas.Index {
	repoIndex := schemas.NewIndex("r_u_d", schemas.IndexType)
	repoIndex.AddColumn("repo_id", "user_id", "is_deleted")

	actUserIndex := schemas.NewIndex("au_r_c_u_d", schemas.IndexType)
	actUserIndex.AddColumn("act_user_id", "repo_id", "created_unix", "user_id", "is_deleted")

	cudIndex := schemas.NewIndex("c_u_d", schemas.IndexType)
	cudIndex.AddColumn("created_unix", "user_id", "is_deleted")

	// old 2-column index, before the migration
	cuIndex := schemas.NewIndex("c_u", schemas.IndexType)
	cuIndex.AddColumn("user_id", "is_deleted")

	actUserUserIndex := schemas.NewIndex("au_c_u", schemas.IndexType)
	actUserUserIndex.AddColumn("act_user_id", "created_unix", "user_id")

	return []*schemas.Index{actUserIndex, repoIndex, cudIndex, cuIndex, actUserUserIndex}
}

func Test_AddCreatedUnixToActionUserIsDeletedIndex(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(actionBeforeV339))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	indexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "action")
	require.NoError(t, err)
	assert.True(t, hasIndexWithColumns(indexes, []string{"user_id", "is_deleted"}, false), "old c_u index should exist before migration")
	assert.False(t, hasIndexWithColumns(indexes, []string{"user_id", "is_deleted", "created_unix"}, false), "new c_u index should not exist before migration")

	require.NoError(t, AddCreatedUnixToActionUserIsDeletedIndex(x))

	indexes, err = x.Dialect().GetIndexes(x.DB(), context.Background(), "action")
	require.NoError(t, err)
	assert.False(t, hasIndexWithColumns(indexes, []string{"user_id", "is_deleted"}, false), "old 2-column c_u index should be gone after migration")
	assert.True(t, hasIndexWithColumns(indexes, []string{"user_id", "is_deleted", "created_unix"}, false), "new 3-column c_u index must exist after migration")
}
