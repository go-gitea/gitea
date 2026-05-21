// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

// actionRunFailureTag mirrors models/actions.ActionRunFailureTag at this migration point.
type actionRunFailureTag struct {
	ID          int64              `xorm:"pk autoincr"`
	RepoID      int64              `xorm:"INDEX UNIQUE(repo_name) NOT NULL"`
	Name        string             `xorm:"VARCHAR(50) UNIQUE(repo_name) NOT NULL"`
	Color       string             `xorm:"VARCHAR(7) NOT NULL DEFAULT ''"`
	Description string             `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func (actionRunFailureTag) TableName() string { return "action_run_failure_tag" }

type actionRunAnalysis struct {
	ID          int64              `xorm:"pk autoincr"`
	AttemptID   int64              `xorm:"UNIQUE NOT NULL"`
	RunID       int64              `xorm:"INDEX NOT NULL"`
	RepoID      int64              `xorm:"INDEX NOT NULL"`
	AuthorID    int64              `xorm:"NOT NULL"`
	Note        string             `xorm:"LONGTEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func (actionRunAnalysis) TableName() string { return "action_run_analysis" }

type actionRunAnalysisTag struct {
	AnalysisID  int64              `xorm:"pk"`
	TagID       int64              `xorm:"pk INDEX"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

func (actionRunAnalysisTag) TableName() string { return "action_run_analysis_tag" }

// AddActionRunAnalysis adds the per-attempt analysis tables plus repo-scoped failure tag taxonomy.
func AddActionRunAnalysis(x db.EngineMigration) error {
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(actionRunFailureTag), new(actionRunAnalysis), new(actionRunAnalysisTag))
	return err
}
