// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func addIssueDependencies(x *xorm.Engine) (err error) {

	type IssueDependency struct {
		ID           int64     `xorm:"pk autoincr"`
		UserID       int64     `xorm:"NOT NULL"`
		IssueID      int64     `xorm:"NOT NULL"`
		DependencyID int64     `xorm:"NOT NULL"`
		Created      time.Time `xorm:"-"`
		CreatedUnix  int64     `xorm:"created"`
		Updated      time.Time `xorm:"-"`
		UpdatedUnix  int64     `xorm:"updated"`
	}

	if err = x.Sync(new(IssueDependency)); err != nil {
		return fmt.Errorf("Error creating issue_dependency_table column definition: %v", err)
	}

	// Update Comment definition
	// This (copied) struct does only contain fields used by xorm as the only use here is to update the database

	// CommentType defines the comment type
	type CommentType int

	// TimeStamp defines a timestamp
	type TimeStamp int64

	type Comment struct {
		ID               int64 `xorm:"pk autoincr"`
		Type             CommentType
		PosterID         int64 `xorm:"INDEX"`
		IssueID          int64 `xorm:"INDEX"`
		LabelID          int64
		OldMilestoneID   int64
		MilestoneID      int64
		OldAssigneeID    int64
		AssigneeID       int64
		OldTitle         string
		NewTitle         string
		DependentIssueID int64

		CommitID int64
		Line     int64
		Content  string `xorm:"TEXT"`

		CreatedUnix TimeStamp `xorm:"INDEX created"`
		UpdatedUnix TimeStamp `xorm:"INDEX updated"`

		// Reference issue in commit message
		CommitSHA string `xorm:"VARCHAR(40)"`
	}

	if err = x.Sync(new(Comment)); err != nil {
		return fmt.Errorf("Error updating issue_comment table column definition: %v", err)
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
