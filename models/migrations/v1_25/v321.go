// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"xorm.io/xorm"
)

type comment struct {
	BeforeCommitID string `xorm:"VARCHAR(64)"`
}

// TableName return database table name for xorm
func (comment) TableName() string {
	return "comment"
}

func AddBeforeCommitIDForComment(x *xorm.Engine) error {
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(comment)); err != nil {
		return err
	}
	_, err := x.Exec("UPDATE comment SET before_commit_id = (SELECT merge_base FROM pull_request WHERE pull_request.issue_id = comment.issue_id) WHERE before_commit_id IS NULL")
	return err
}
