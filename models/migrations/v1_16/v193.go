// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16 //nolint

import (
	"xorm.io/xorm"
)

func AddRepoIDForAttachment(x *xorm.Engine) error {
	type Attachment struct {
		ID         int64  `xorm:"pk autoincr"`
		UUID       string `xorm:"uuid UNIQUE"`
		RepoID     int64  `xorm:"INDEX"` // this should not be zero
		IssueID    int64  `xorm:"INDEX"` // maybe zero when creating
		ReleaseID  int64  `xorm:"INDEX"` // maybe zero when creating
		UploaderID int64  `xorm:"INDEX DEFAULT 0"`
	}
	if err := x.Sync(new(Attachment)); err != nil {
		return err
	}

	if _, err := x.Exec("UPDATE `attachment` set repo_id = (SELECT repo_id FROM `issue` WHERE `issue`.id = `attachment`.issue_id) WHERE `attachment`.issue_id > 0"); err != nil {
		return err
	}

	if _, err := x.Exec("UPDATE `attachment` set repo_id = (SELECT repo_id FROM `release` WHERE `release`.id = `attachment`.release_id) WHERE `attachment`.release_id > 0"); err != nil {
		return err
	}

	return nil
}
