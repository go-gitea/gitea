// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/setting"
	"fmt"
	"github.com/go-xorm/xorm"
	"strconv"
	"time"
)

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

func addTimeTracking(x *xorm.Engine) error {
	if err := x.Sync2(new(Stopwatch)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	if err := x.Sync2(new(TrackedTime)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	x.Where("type = ?", V16UnitTypeIssues).Iterate(new(RepoUnit), func(id int, bean interface{}) error {
		unit := bean.(*RepoUnit)
		changes := false
		if _, ok := unit.Config["EnableTimetracker"]; !ok {
			unit.Config["EnableTimetracker"] = strconv.FormatBool(setting.Service.DefaultEnableTimetracking)
			changes = true
		}
		if _, ok := unit.Config["AllowOnlyContributorsToTrackTime"]; !ok {
			unit.Config["AllowOnlyContributorsToTrackTime"] = strconv.FormatBool(setting.Service.DefaultAllowOnlyContributorsToTrackTime)
			changes = true
		}
		if changes {
			_, err := x.Id(id).Cols("config").Update(unit)
			return err
		}
		return nil
	})
	return nil
}
