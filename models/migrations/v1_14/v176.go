// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import (
	"xorm.io/xorm"
)

// RemoveInvalidLabels looks through the database to look for comments and issue_labels
// that refer to labels do not belong to the repository or organization that repository
// that the issue is in
func RemoveInvalidLabels(x *xorm.Engine) error {
	type Comment struct {
		ID      int64 `xorm:"pk autoincr"`
		Type    int   `xorm:"INDEX"`
		IssueID int64 `xorm:"INDEX"`
		LabelID int64
	}

	type Issue struct {
		ID     int64 `xorm:"pk autoincr"`
		RepoID int64 `xorm:"INDEX UNIQUE(repo_index)"`
		Index  int64 `xorm:"UNIQUE(repo_index)"` // Index in one repository.
	}

	type Repository struct {
		ID        int64  `xorm:"pk autoincr"`
		OwnerID   int64  `xorm:"UNIQUE(s) index"`
		LowerName string `xorm:"UNIQUE(s) INDEX NOT NULL"`
	}

	type Label struct {
		ID     int64 `xorm:"pk autoincr"`
		RepoID int64 `xorm:"INDEX"`
		OrgID  int64 `xorm:"INDEX"`
	}

	type IssueLabel struct {
		ID      int64 `xorm:"pk autoincr"`
		IssueID int64 `xorm:"UNIQUE(s)"`
		LabelID int64 `xorm:"UNIQUE(s)"`
	}

	if err := x.Sync(new(Comment), new(Issue), new(Repository), new(Label), new(IssueLabel)); err != nil {
		return err
	}

	if _, err := x.Exec(`DELETE FROM issue_label WHERE issue_label.id IN (
		SELECT il_too.id FROM (
			SELECT il_too_too.id
				FROM issue_label AS il_too_too
					INNER JOIN label ON il_too_too.label_id = label.id
					INNER JOIN issue on issue.id = il_too_too.issue_id
					INNER JOIN repository on repository.id = issue.repo_id
				WHERE
					(label.org_id = 0 AND issue.repo_id != label.repo_id) OR (label.repo_id = 0 AND label.org_id != repository.owner_id)
	) AS il_too )`); err != nil {
		return err
	}

	if _, err := x.Exec(`DELETE FROM comment WHERE comment.id IN (
		SELECT il_too.id FROM (
			SELECT com.id
				FROM comment AS com
					INNER JOIN label ON com.label_id = label.id
					INNER JOIN issue on issue.id = com.issue_id
					INNER JOIN repository on repository.id = issue.repo_id
				WHERE
					com.type = ? AND ((label.org_id = 0 AND issue.repo_id != label.repo_id) OR (label.repo_id = 0 AND label.org_id != repository.owner_id))
	) AS il_too)`, 7); err != nil {
		return err
	}

	return nil
}
