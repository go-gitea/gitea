// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/util"

	"github.com/go-xorm/xorm"
)

func addRepoTransfer(x *xorm.Engine) error {
	type RepoTransfer struct {
		ID          int64 `xorm:"pk autoincr"`
		UserID      int64
		RecipientID int64
		RepoID      int64
		CreatedUnix util.TimeStamp `xorm:"INDEX NOT NULL created"`
		UpdatedUnix util.TimeStamp `xorm:"INDEX NOT NULL updated"`
		Status      bool
	}

	return x.Sync(new(RepoTransfer))
}
