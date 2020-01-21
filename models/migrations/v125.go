// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models"

	"xorm.io/xorm"
)

func addLockedResourceTable(x *xorm.Engine) error {

	type LockedResource struct {
		LockType string `xorm:"pk VARCHAR(30)"`
		LockKey  int64  `xorm:"pk"`
		Counter  int64  `xorm:"NOT NULL DEFAULT 0"`
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync2(new(LockedResource)); err != nil {
		return err
	}

	// Remove data we're goint to rebuild
	if _, err := sess.Delete(&LockedResource{LockType: models.IssueLockedEnumerator}); err != nil {
		return err
	}

	// Create current data for all repositories with issues and PRs
	if _, err := sess.Exec("INSERT INTO locked_resource (lock_type, lock_key, counter) "+
		"SELECT ?, max_data.repo_id, max_data.max_index "+
		"FROM ( SELECT issue.repo_id AS repo_id, max(issue.index) AS max_index "+
		"FROM issue GROUP BY issue.repo_id) AS max_data",
		models.IssueLockedEnumerator); err != nil {
		return err
	}

	return sess.Commit()
}
