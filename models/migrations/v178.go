// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addCommentOnCommit(x *xorm.Engine) error {
	type Comment struct {
		ID        int64  `xorm:"pk autoincr"`
		RepoID    int64  `xorm:"INDEX"`
		CommitSHA string `xorm:"VARCHAR(40)"`
	}
	// TODO: remove commit_id column since it's unused.
	if err := x.Sync2(new(Comment)); err != nil {
		return err
	}

	_, err := x.Exec("UPDATE comment SET repo_id = (SELECT repo_id FROM issue WHERE issue.id = comment.issue_id)")
	return err
}
