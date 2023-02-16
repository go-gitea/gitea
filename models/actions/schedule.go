// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
	webhook_module "code.gitea.io/gitea/modules/webhook"
)

// ActionSchedule represents a schedule of a workflow file
type ActionSchedule struct {
	ID            int64
	Title         string
	Specs         []string
	EntryIDs      []int                  `xorm:"entry_ids"`
	RepoID        int64                  `xorm:"index"`
	Repo          *repo_model.Repository `xorm:"-"`
	OwnerID       int64                  `xorm:"index"`
	WorkflowID    string                 `xorm:"index"` // the name of workflow file
	TriggerUserID int64
	TriggerUser   *user_model.User `xorm:"-"`
	Ref           string
	CommitSHA     string
	Event         webhook_module.HookEventType
	EventPayload  string `xorm:"LONGTEXT"`
	Content       []byte
	Created       timeutil.TimeStamp `xorm:"created"`
	Updated       timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ActionSchedule))
}

// CreateScheduleTask creates new schedule task.
func CreateScheduleTask(ctx context.Context, rows []*ActionSchedule) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if len(rows) > 0 {
		if err = db.Insert(ctx, rows); err != nil {
			return err
		}
	}

	return committer.Commit()
}

func DeleteScheduleTaskByRepo(ctx context.Context, id int64) error {
	if _, err := db.GetEngine(ctx).Delete(&ActionSchedule{RepoID: id}); err != nil {
		return err
	}
	return nil
}

func UpdateSchedule(ctx context.Context, schedule *ActionSchedule, cols ...string) error {
	sess := db.GetEngine(ctx).ID(schedule.ID)
	if len(cols) > 0 {
		sess.Cols(cols...)
	}
	_, err := sess.Update(schedule)

	return err
}
