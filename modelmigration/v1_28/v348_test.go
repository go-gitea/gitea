// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import (
	"context"
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

// reactionBeforeV348 mirrors the pre-migration reaction table: no release_id
// column and a 6-column UNIQUE(s) index on issue/comment reactions.
type reactionBeforeV348 struct {
	ID               int64  `xorm:"pk autoincr"`
	Type             string `xorm:"UNIQUE(s) NOT NULL"`
	IssueID          int64  `xorm:"UNIQUE(s) NOT NULL DEFAULT(0)"`
	CommentID        int64  `xorm:"UNIQUE(s) NOT NULL DEFAULT(0)"`
	UserID           int64  `xorm:"UNIQUE(s) NOT NULL"`
	OriginalAuthorID int64  `xorm:"UNIQUE(s) NOT NULL DEFAULT(0)"`
	OriginalAuthor   string `xorm:"UNIQUE(s)"`
}

func (reactionBeforeV348) TableName() string { return "reaction" }

func Test_AddReleaseIDToReaction(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(reactionBeforeV348))
	defer deferable()

	// pre-existing rows in the old schema must survive the migration
	_, err := x.Insert(&reactionBeforeV348{Type: "heart", IssueID: 1, UserID: 1})
	require.NoError(t, err)
	_, err = x.Insert(&reactionBeforeV348{Type: "laugh", CommentID: 2, UserID: 1})
	require.NoError(t, err)

	indexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "reaction")
	require.NoError(t, err)
	oldIdx, ok := indexes["s"] // GetIndexes strips the UQE_reaction_ prefix
	require.True(t, ok, "old 6-column unique s index should exist before migration")
	assert.Equal(t, schemas.UniqueType, oldIdx.Type)
	assert.Equal(t, []string{"type", "issue_id", "comment_id", "user_id", "original_author_id", "original_author"}, oldIdx.Cols)

	require.NoError(t, AddReleaseIDToReaction(t.Context(), x))
	require.NoError(t, AddReleaseIDToReaction(t.Context(), x)) // idempotent

	indexes, err = x.Dialect().GetIndexes(x.DB(), context.Background(), "reaction")
	require.NoError(t, err)
	newIdx, ok := indexes["s"]
	require.True(t, ok, "new unique s index should exist after migration")
	assert.Equal(t, schemas.UniqueType, newIdx.Type)
	assert.Equal(t, []string{"type", "issue_id", "comment_id", "release_id", "user_id", "original_author_id", "original_author"}, newIdx.Cols)

	// the new index is unique per target object: a release reaction distinct from
	// the issue/comment reactions above is allowed, an exact duplicate is rejected
	_, err = x.Exec("INSERT INTO reaction (type, issue_id, comment_id, release_id, user_id, original_author_id, original_author) VALUES ('heart', 0, 0, 3, 1, 0, '')")
	require.NoError(t, err)
	_, err = x.Exec("INSERT INTO reaction (type, issue_id, comment_id, release_id, user_id, original_author_id, original_author) VALUES ('heart', 0, 0, 3, 1, 0, '')")
	require.Error(t, err)
	_, err = x.Exec("INSERT INTO reaction (type, issue_id, comment_id, release_id, user_id, original_author_id, original_author) VALUES ('heart', 0, 0, 3, 2, 0, '')")
	require.NoError(t, err)
}
