// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/setting"
	"fmt"
	"github.com/go-xorm/xorm"
	"time"
)

func addIssueDependencies(x *xorm.Engine) (err error) {

	type IssueDependency struct {
		ID           int64     `xorm:"pk autoincr"`
		UserID       int64     `xorm:"NOT NULL"`
		IssueID      int64     `xorm:"NOT NULL"`
		DependencyID int64     `xorm:"NOT NULL"`
		Created      time.Time `xorm:"-"`
		CreatedUnix  int64     `xorm:"INDEX created"`
		Updated      time.Time `xorm:"-"`
		UpdatedUnix  int64     `xorm:"updated"`
	}

	if err = x.Sync(new(IssueDependency)); err != nil {
		return fmt.Errorf("Error creating issue_dependency_table column definition: %v", err)
	}

	// RepoUnit describes all units of a repository
	type RepoUnit struct {
		ID          int64
		RepoID      int64                  `xorm:"INDEX(s)"`
		Type        int                    `xorm:"INDEX(s)"`
		Config      map[string]interface{} `xorm:"JSON"`
		CreatedUnix int64                  `xorm:"INDEX CREATED"`
		Created     time.Time              `xorm:"-"`
	}

	//Updating existing issue units
	units := make([]*RepoUnit, 0, 100)
	err = x.Where("`type` = ?", V16UnitTypeIssues).Find(&units)
	if err != nil {
		return fmt.Errorf("Query repo units: %v", err)
	}
	for _, unit := range units {
		if unit.Config == nil {
			unit.Config = make(map[string]interface{})
		}
		if _, ok := unit.Config["EnableDependencies"]; !ok {
			unit.Config["EnableDependencies"] = setting.Service.DefaultEnableDependencies
		}
		if _, err := x.ID(unit.ID).Cols("config").Update(unit); err != nil {
			return err
		}
	}

	return err
}
