// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MITstyle
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func createSecretsTable(x *xorm.Engine) error {
	type Secret struct {
		ID          int64
		UserID      int64              `xorm:"index NOTNULL"`
		RepoID      int64              `xorm:"index NOTNULL"`
		Name        string             `xorm:"NOTNULL"`
		Data        string             `xorm:"TEXT"`
		PullRequest bool               `xorm:"NOTNULL"`
		CreatedUnix timeutil.TimeStamp `xorm:"created NOTNULL"`
	}

	return x.Sync(new(Secret))
}
