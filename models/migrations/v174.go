// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func addRepoTransfer(x *xorm.Engine) error {
	type RepoTransfer struct {
		ID          int64 `xorm:"pk autoincr"`
		DoerID      int64
		RecipientID int64
		RepoID      int64
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX NOT NULL created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX NOT NULL updated"`
		TeamIDs     []int64
	}

	return x.Sync(new(RepoTransfer))
}
