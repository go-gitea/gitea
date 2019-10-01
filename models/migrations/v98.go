// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/go-xorm/xorm"
)

func addProjectsTable(x *xorm.Engine) error {

	type ProjectType uint8

	type Project struct {
		ID              int64  `xorm:"pk autoincr"`
		Title           string `xorm:"INDEX NOT NULL"`
		Description     string `xorm:"TEXT"`
		RepoID          int64  `xorm:"NOT NULL"`
		CreatorID       int64  `xorm:"NOT NULL"`
		IsClosed        bool   `xorm:"INDEX"`
		NumIssues       int
		NumClosedIssues int

		Type ProjectType

		ClosedDateUnix timeutil.TimeStamp
		CreatedUnix    timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix    timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	return x.Sync(new(Project))
}
