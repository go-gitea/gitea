// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16 //nolint

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
)

func Test_AddRepoIDForAttachment(t *testing.T) {
	type Attachment struct {
		ID         int64  `xorm:"pk autoincr"`
		UUID       string `xorm:"uuid UNIQUE"`
		IssueID    int64  `xorm:"INDEX"` // maybe zero when creating
		ReleaseID  int64  `xorm:"INDEX"` // maybe zero when creating
		UploaderID int64  `xorm:"INDEX DEFAULT 0"`
	}

	type Issue struct {
		ID     int64
		RepoID int64
	}

	type Release struct {
		ID     int64
		RepoID int64
	}

	// Prepare and load the testing database
	x, deferrable := base.PrepareTestEnv(t, 0, new(Attachment), new(Issue), new(Release))
	defer deferrable()
	if x == nil || t.Failed() {
		return
	}

	// Run the migration
	if err := AddRepoIDForAttachment(x); err != nil {
		assert.NoError(t, err)
		return
	}

	type NewAttachment struct {
		ID         int64  `xorm:"pk autoincr"`
		UUID       string `xorm:"uuid UNIQUE"`
		RepoID     int64  `xorm:"INDEX"` // this should not be zero
		IssueID    int64  `xorm:"INDEX"` // maybe zero when creating
		ReleaseID  int64  `xorm:"INDEX"` // maybe zero when creating
		UploaderID int64  `xorm:"INDEX DEFAULT 0"`
	}

	var issueAttachments []*NewAttachment
	err := x.Table("attachment").Where("issue_id > 0").Find(&issueAttachments)
	assert.NoError(t, err)
	for _, attach := range issueAttachments {
		assert.Positive(t, attach.RepoID)
		assert.Positive(t, attach.IssueID)
		var issue Issue
		has, err := x.ID(attach.IssueID).Get(&issue)
		assert.NoError(t, err)
		assert.True(t, has)
		assert.EqualValues(t, attach.RepoID, issue.RepoID)
	}

	var releaseAttachments []*NewAttachment
	err = x.Table("attachment").Where("release_id > 0").Find(&releaseAttachments)
	assert.NoError(t, err)
	for _, attach := range releaseAttachments {
		assert.Positive(t, attach.RepoID)
		assert.Positive(t, attach.ReleaseID)
		var release Release
		has, err := x.ID(attach.ReleaseID).Get(&release)
		assert.NoError(t, err)
		assert.True(t, has)
		assert.EqualValues(t, attach.RepoID, release.RepoID)
	}
}
