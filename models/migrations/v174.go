// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addRepoTransfer(x *xorm.Engine) error {
	type RepoTransfer struct {
		ID          int64 `xorm:"pk autoincr"`
		DoerID      int64
		RecipientID int64
		RepoID      int64
		TeamIDs     []int64
		CreatedUnix int64 `xorm:"INDEX NOT NULL created"`
		UpdatedUnix int64 `xorm:"INDEX NOT NULL updated"`
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync2(new(RepoTransfer)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	return sess.Commit()
}
