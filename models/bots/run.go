// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"fmt"
	"hash/fnv"

	"code.gitea.io/gitea/core"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/nektos/act/pkg/jobparser"
)

// Run represents a run of a workflow file
type Run struct {
	ID            int64
	Name          string
	RepoID        int64  `xorm:"index unique(repo_workflow_index)"`
	WorkflowID    string `xorm:"index unique(repo_workflow_index)"` // the name of workflow file
	Index         int64  `xorm:"index unique(repo_workflow_index)"` // a unique number for each run of a particular workflow in a repository
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
	db.RegisterModel(new(RunIndex))
}

func (Run) TableName() string {
	return "bots_run"
}

// InsertRun inserts a bot run
func InsertRun(run *Run, jobs []*jobparser.SingleWorkflow) error {
	var groupId int64
	{
		// tricky way to get resource group id
		h := fnv.New64()
		_, _ = h.Write([]byte(fmt.Sprintf("%d_%s", run.RepoID, run.WorkflowID)))
		groupId = int64(h.Sum64())
	}

	index, err := db.GetNextResourceIndex("bots_run_index", groupId)
	if err != nil {
		return err
	}
	run.Index = index

	ctx, commiter, err := db.TxContext()
	if err != nil {
		return err
	}
	defer commiter.Close()

	if err := db.Insert(ctx, run); err != nil {
		return err
	}

	runJobs := make([]*RunJob, 0, len(jobs))
	for _, v := range jobs {
		_, job := v.Job()
		payload, _ := v.Marshal()
		runJobs = append(runJobs, &RunJob{
			RunID:           run.ID,
			Name:            job.Name,
			WorkflowPayload: payload,
			Needs:           nil, // TODO: analyse needs
			RunsOn:          job.RunsOn(),
			TaskID:          0,
			Status:          core.StatusPending,
		})
	}
	if err := db.Insert(ctx, runJobs); err != nil {
		return err
	}

	if err := commiter.Commit(); err != nil {
		return err
	}

	return nil
}

type RunIndex db.ResourceIndex

func (RunIndex) TableName() string {
	return "bots_run_index"
}
