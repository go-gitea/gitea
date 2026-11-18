// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddCommitComment(t *testing.T) {
	type Attachment struct {
		ID   int64 `xorm:"pk autoincr"`
		UUID string
	}

	x, deferrable := migrationtest.PrepareTestEnv(t, 0, new(Attachment))
	defer deferrable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&Attachment{UUID: "a"})
	require.NoError(t, err)
	require.NoError(t, AddCommitComment(t.Context(), x))

	var commitCommentIDs []int64
	require.NoError(t, x.Table("attachment").Cols("commit_comment_id").Find(&commitCommentIDs))
	assert.Equal(t, []int64{0}, commitCommentIDs)

	has, err := x.IsTableExist("commit_comment")
	require.NoError(t, err)
	assert.True(t, has)
}
