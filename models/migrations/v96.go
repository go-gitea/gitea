// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "github.com/go-xorm/xorm"

func addProjectIDToCommentsTable(x *xorm.Engine) error {

	sess := x.NewSession()
	defer sess.Close()

	type Comment struct {
		OldProjectID int64
		ProjectID    int64
	}

	if err := sess.Sync2(new(Comment)); err != nil {
		return err
	}

	return sess.Commit()
}
