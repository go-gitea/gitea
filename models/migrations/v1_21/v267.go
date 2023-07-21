// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/timeutil"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"xorm.io/xorm"
)

// UpdateActionsRefIndex updates the index of actions ref field
func UpdateActionsRefIndex(x *xorm.Engine) error {
	type ActionRun struct {
		ID                int64
		Title             string
		RepoID            int64  `xorm:"index unique(repo_index)"`
		OwnerID           int64  `xorm:"index"`
		WorkflowID        string `xorm:"index"`                    // the name of workflow file
		Index             int64  `xorm:"index unique(repo_index)"` // a unique number for each run of a repository
		TriggerUserID     int64  `xorm:"index"`
		Ref               string `xorm:"index"` // the ref of the run
		CommitSHA         string
		IsForkPullRequest bool                         // If this is triggered by a PR from a forked repository or an untrusted user, we need to check if it is approved and limit permissions when running the workflow.
		NeedApproval      bool                         // may need approval if it's a fork pull request
		ApprovedBy        int64                        `xorm:"index"` // who approved
		Event             webhook_module.HookEventType // the webhook event that causes the workflow to run
		EventPayload      string                       `xorm:"LONGTEXT"`
		TriggerEvent      string                       // the trigger event defined in the `on` configuration of the triggered workflow
		Status            actions.Status               `xorm:"index"`
		Started           timeutil.TimeStamp
		Stopped           timeutil.TimeStamp
		Created           timeutil.TimeStamp `xorm:"created"`
		Updated           timeutil.TimeStamp `xorm:"updated"`
	}
	return x.Sync(new(ActionRun))
}
