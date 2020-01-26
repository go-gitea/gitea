// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func addTimetracking(x *xorm.Engine) error {
	// RepoUnit describes all units of a repository
	type RepoUnit struct {
		ID          int64
		RepoID      int64                  `xorm:"INDEX(s)"`
		Type        int                    `xorm:"INDEX(s)"`
		Config      map[string]interface{} `xorm:"JSON"`
		CreatedUnix int64                  `xorm:"INDEX CREATED"`
		Created     time.Time              `xorm:"-"`
	}

	// Stopwatch see models/issue_stopwatch.go
	type Stopwatch struct {
		ID          int64     `xorm:"pk autoincr"`
		IssueID     int64     `xorm:"INDEX"`
		UserID      int64     `xorm:"INDEX"`
		Created     time.Time `xorm:"-"`
		CreatedUnix int64
	}

	// TrackedTime see models/issue_tracked_time.go
	type TrackedTime struct {
		ID          int64     `xorm:"pk autoincr" json:"id"`
		IssueID     int64     `xorm:"INDEX" json:"issue_id"`
		UserID      int64     `xorm:"INDEX" json:"user_id"`
		Created     time.Time `xorm:"-" json:"created"`
		CreatedUnix int64     `json:"-"`
		Time        int64     `json:"time"`
	}

	if err := x.Sync2(new(Stopwatch)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	if err := x.Sync2(new(TrackedTime)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	//Updating existing issue units
	units := make([]*RepoUnit, 0, 100)
	err := x.Where("`type` = ?", V16UnitTypeIssues).Find(&units)
	if err != nil {
		return fmt.Errorf("Query repo units: %v", err)
	}
	for _, unit := range units {
		if unit.Config == nil {
			unit.Config = make(map[string]interface{})
		}
		if _, ok := unit.Config["EnableTimetracker"]; !ok {
			unit.Config["EnableTimetracker"] = setting.Service.DefaultEnableTimetracking
		}
		if _, ok := unit.Config["AllowOnlyContributorsToTrackTime"]; !ok {
			unit.Config["AllowOnlyContributorsToTrackTime"] = setting.Service.DefaultAllowOnlyContributorsToTrackTime
		}
		if _, err := x.ID(unit.ID).Cols("config").Update(unit); err != nil {
			return err
		}
	}
	return nil
}
