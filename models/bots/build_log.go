// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// BuildLog represents a build's log, every build has a standalone table
type BuildLog struct {
	ID      int64
	StepID  int64              `xorm:"index"`
	Content string             `xorm:"BINARY"`
	Created timeutil.TimeStamp `xorm:"created"`
}

func init() {
	db.RegisterModel(new(BuildLog))
}

func GetBuildLogTableName(buildID int64) string {
	return fmt.Sprintf("bots_build_log_%d", buildID)
}

// CreateBuildLog table for a build
func CreateBuildLog(buildID int64) error {
	return db.GetEngine(db.DefaultContext).
		Table(GetBuildLogTableName(buildID)).
		Sync2(new(BuildLog))
}

func GetBuildLogs(buildID, jobID int64) (logs []BuildLog, err error) {
	err = db.GetEngine(db.DefaultContext).Table(GetBuildLogTableName(buildID)).
		Where("build_step_id=?", jobID).
		Find(&logs)
	return
}
