// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func createProtectedTagTable(x *xorm.Engine) error {
	type ProtectedTag struct {
		ID               int64   `xorm:"pk autoincr"`
		RepoID           int64   `xorm:"UNIQUE(s)"`
		NamePattern      string  `xorm:"UNIQUE(s)"`
		WhitelistUserIDs []int64 `xorm:"JSON TEXT"`
		WhitelistTeamIDs []int64 `xorm:"JSON TEXT"`

		CreatedUnix timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync2(new(ProtectedTag)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	return sess.Commit()
}
