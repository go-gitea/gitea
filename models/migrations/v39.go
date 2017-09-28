// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"encoding/json"
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/setting"

	"github.com/go-xorm/core"
	"github.com/go-xorm/xorm"
)

// IssuesConfigV39 describes issues config
type IssuesConfigV39 struct {
	EnableTimetracker                bool
	AllowOnlyContributorsToTrackTime bool
}

// FromDB fills up a IssuesConfigV39 from serialized format.
func (cfg *IssuesConfigV39) FromDB(bs []byte) error {
	return json.Unmarshal(bs, &cfg)
}

// ToDB exports a IssuesConfigV39 to a serialized format.
func (cfg *IssuesConfigV39) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
}

func addTimetracking(x *xorm.Engine) error {
	// RepoUnit describes all units of a repository
	type RepoUnit struct {
		ID          int64
		RepoID      int64 `xorm:"INDEX(s)"`
		Type        int   `xorm:"INDEX(s)"`
		Index       int
		Config      core.Conversion `xorm:"TEXT"`
		CreatedUnix int64           `xorm:"INDEX CREATED"`
		Created     time.Time       `xorm:"-"`
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
	x.Where("`type` = ?", V16UnitTypeIssues).Find(units)
	for _, unit := range units {
		if unit.Config != nil {
			continue
		}
		unit.Config = &IssuesConfigV39{
			EnableTimetracker:                setting.Service.DefaultEnableTimetracking,
			AllowOnlyContributorsToTrackTime: setting.Service.DefaultAllowOnlyContributorsToTrackTime,
		}
		if _, err := x.Id(unit.ID).Cols("config").Update(unit); err != nil {
			return err
		}
	}
	return nil
}
