// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/require"
)

func Test_FixMissedRepoIDWhenMigrateAttachments(t *testing.T) {
	type Attachment struct {
		ID            int64  `xorm:"pk autoincr"`
		UUID          string `xorm:"uuid UNIQUE"`
		RepoID        int64  `xorm:"INDEX"`           // this should not be zero
		IssueID       int64  `xorm:"INDEX"`           // maybe zero when creating
		ReleaseID     int64  `xorm:"INDEX"`           // maybe zero when creating
		UploaderID    int64  `xorm:"INDEX DEFAULT 0"` // Notice: will be zero before this column added
		CommentID     int64  `xorm:"INDEX"`
		Name          string
		DownloadCount int64              `xorm:"DEFAULT 0"`
		Size          int64              `xorm:"DEFAULT 0"`
		CreatedUnix   timeutil.TimeStamp `xorm:"created"`
	}

	type Issue struct {
		ID     int64 `xorm:"pk autoincr"`
		RepoID int64 `xorm:"INDEX"`
	}

	type Release struct {
		ID     int64 `xorm:"pk autoincr"`
		RepoID int64 `xorm:"INDEX"`
	}

	// Prepare and load the testing database
	x, deferrable := base.PrepareTestEnv(t, 0, new(Attachment), new(Issue), new(Release))
	defer deferrable()

	require.NoError(t, FixMissedRepoIDWhenMigrateAttachments(x))
}
