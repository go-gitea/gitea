// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm"
)

func AddScopedWorkflowsSchema(x db.EngineMigration) error {
	// Create the action_scoped_workflow_source table
	type ScopedWorkflowConfig struct {
		Required bool     `json:"required"`
		Patterns []string `json:"patterns"`
	}
	type ActionScopedWorkflowSource struct {
		ID              int64                            `xorm:"pk autoincr"`
		OwnerID         int64                            `xorm:"UNIQUE(owner_repo) NOT NULL DEFAULT 0"`
		SourceRepoID    int64                            `xorm:"INDEX UNIQUE(owner_repo) NOT NULL DEFAULT 0"`
		WorkflowConfigs map[string]*ScopedWorkflowConfig `xorm:"JSON TEXT 'workflow_configs'"`
		CreatedUnix     timeutil.TimeStamp               `xorm:"created"`
		UpdatedUnix     timeutil.TimeStamp               `xorm:"updated"`
	}
	if err := x.Sync(new(ActionScopedWorkflowSource)); err != nil {
		return err
	}

	// Add the columns that record where a run's workflow content came from
	type ActionRun struct {
		WorkflowRepoID    int64  `xorm:"NOT NULL DEFAULT 0"`
		WorkflowCommitSHA string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
		IsScopedRun       bool   `xorm:"NOT NULL DEFAULT false"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ActionRun))
	return err
}
