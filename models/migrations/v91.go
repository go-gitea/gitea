// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/util"

	"github.com/go-xorm/xorm"
)

func addProjectsTable(x *xorm.Engine) error {

	type Project struct {
		ID          int64  `xorm:"pk autoincr"`
		Title       string `xorm:"INDEX NOT NULL"`
		Description string `xorm:"NOT NULL"`
		RepoID      string `xorm:"NOT NULL"`
		CreatorID   int64  `xorm:"NOT NULL"`

		CreatedUnix util.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix util.TimeStamp `xorm:"INDEX updated"`
	}

	return x.Sync(new(Project))
}
