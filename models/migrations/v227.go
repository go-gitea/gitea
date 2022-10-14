// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func createSecretsTable(x *xorm.Engine) error {
	type Secret struct {
		ID          int64
		UserID      int64 `xorm:"index"`
		RepoID      int64 `xorm:"index"`
		Name        string
		Data        string
		PullRequest bool
		CreatedUnix timeutil.TimeStamp `xorm:"created"`
	}

	return x.Sync(new(Secret))
}
