// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func addBotTables(x *xorm.Engine) error {
	type BotRunner struct {
		ID          int64
		UUID        string `xorm:"CHAR(36) UNIQUE"`
		Name        string `xorm:"VARCHAR(32) UNIQUE"`
		OS          string `xorm:"VARCHAR(16) index"` // the runner running os
		Arch        string `xorm:"VARCHAR(16) index"` // the runner running architecture
		Type        string `xorm:"VARCHAR(16)"`
		OwnerID     int64  `xorm:"index"` // org level runner, 0 means system
		RepoID      int64  `xorm:"index"` // repo level runner, if orgid also is zero, then it's a global
		Description string `xorm:"TEXT"`
		Base        int    // 0 native 1 docker 2 virtual machine
		RepoRange   string // glob match which repositories could use this runner
		Token       string
		LastOnline  timeutil.TimeStamp
		Created     timeutil.TimeStamp `xorm:"created"`
	}

	type BotTask struct {
		ID           int64
		UUID         string `xorm:"CHAR(36)"`
		RepoID       int64  `xorm:"index"`
		Type         string `xorm:"VARCHAR(16)"` // 0 commit 1 pullrequest
		Ref          string
		CommitSHA    string
		Event        string
		Token        string // token for this task
		Grant        string // permissions for this task
		EventPayload string `xorm:"LONGTEXT"`
		RunnerID     int64  `xorm:"index"`
		Status       int
		Content      string             `xorm:"LONGTEXT"`
		Created      timeutil.TimeStamp `xorm:"created"`
		StartTime    timeutil.TimeStamp
		EndTime      timeutil.TimeStamp
		Updated      timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync2(new(BotRunner), new(BotTask))
}
