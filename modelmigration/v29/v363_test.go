// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type reactionBeforeV363 struct {
	ID               int64              `xorm:"pk autoincr"`
	Type             string             `xorm:"INDEX UNIQUE(s) NOT NULL"`
	IssueID          int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
	CommentID        int64              `xorm:"INDEX UNIQUE(s)"`
	UserID           int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
	OriginalAuthorID int64              `xorm:"INDEX UNIQUE(s) NOT NULL DEFAULT(0)"`
	OriginalAuthor   string             `xorm:"INDEX UNIQUE(s)"`
	CreatedUnix      timeutil.TimeStamp `xorm:"INDEX created"`
}

func (reactionBeforeV363) TableName() string { return "reaction" }

func Test_AddReleaseIDToReaction(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(reactionBeforeV363))
	defer deferable()

	_, err := x.Insert(&reactionBeforeV363{Type: "heart", IssueID: 1, UserID: 1})
	require.NoError(t, err)

	require.NoError(t, AddReleaseIDToReaction(t.Context(), x))
	require.NoError(t, AddReleaseIDToReaction(t.Context(), x))

	insertReleaseReaction := func(releaseID int64) error {
		_, err := x.Exec("INSERT INTO reaction (type, issue_id, comment_id, release_id, user_id, original_author_id, original_author) VALUES ('heart', 0, 0, ?, 1, 0, '')", releaseID)
		return err
	}
	require.NoError(t, insertReleaseReaction(3))
	require.NoError(t, insertReleaseReaction(4))
	require.Error(t, insertReleaseReaction(4))

	count, err := x.Where("issue_id = 1 AND release_id = 0").Count(new(reactionBeforeV363))
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}
