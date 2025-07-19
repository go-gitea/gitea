// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func Test_AddFileStatusToAttachment(t *testing.T) {
	type Attachment struct {
		ID                int64  `xorm:"pk autoincr"`
		UUID              string `xorm:"uuid UNIQUE"`
		RepoID            int64  `xorm:"INDEX"`           // this should not be zero
		IssueID           int64  `xorm:"INDEX"`           // maybe zero when creating
		ReleaseID         int64  `xorm:"INDEX"`           // maybe zero when creating
		UploaderID        int64  `xorm:"INDEX DEFAULT 0"` // Notice: will be zero before this column added
		CommentID         int64  `xorm:"INDEX"`
		Name              string
		DownloadCount     int64              `xorm:"DEFAULT 0"`
		Size              int64              `xorm:"DEFAULT 0"`
		CreatedUnix       timeutil.TimeStamp `xorm:"created"`
		CustomDownloadURL string             `xorm:"-"`
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(Attachment))
	defer deferable()

	assert.NoError(t, AddFileStatusToAttachment(x))
}
