// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"code.gitea.io/gitea/core"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/timeutil"
)

// Run represents a run of a workflow file
type Run struct {
	ID            int64
	Name          string
	RepoID        int64  `xorm:"index unique(repo_workflow_number)"`
	WorkflowID    string `xorm:"index unique(repo_workflow_number)"` // the name of workflow file
	Number        int64  `xorm:"index unique(repo_workflow_number)"` // a unique number for each run of a particular workflow in a repository
	TriggerUserID int64
	TriggerUser   *user_model.User `xorm:"-"`
	Ref           string
	CommitSHA     string
	Event         webhook.HookEventType
	Token         string           // token for this task
	Grant         string           // permissions for this task
	EventPayload  string           `xorm:"LONGTEXT"`
	Status        core.BuildStatus `xorm:"index"`
	StartTime     timeutil.TimeStamp
	EndTime       timeutil.TimeStamp
	Created       timeutil.TimeStamp `xorm:"created"`
	Updated       timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(Run))
}

func (Run) TableName() string {
	return "bots_run"
}
