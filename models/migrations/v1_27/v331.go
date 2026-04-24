// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"context"
	"time"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

type actionRunAttempt struct {
	ID                int64
	RepoID            int64 `xorm:"index(repo_concurrency_status)"`
	RunID             int64 `xorm:"UNIQUE(run_attempt)"`
	Attempt           int64 `xorm:"UNIQUE(run_attempt)"`
	TriggerUserID     int64
	ConcurrencyGroup  string `xorm:"index(repo_concurrency_status) NOT NULL DEFAULT ''"`
	ConcurrencyCancel bool   `xorm:"NOT NULL DEFAULT FALSE"`
	Status            int    `xorm:"index(repo_concurrency_status)"`
	Started           timeutil.TimeStamp
	Stopped           timeutil.TimeStamp
	Created           timeutil.TimeStamp `xorm:"created"`
	Updated           timeutil.TimeStamp `xorm:"updated"`
}

func (actionRunAttempt) TableName() string {
	return "action_run_attempt"
}

type actionArtifact struct {
	ID                 int64 `xorm:"pk autoincr"`
	RunID              int64 `xorm:"index unique(runid_attempt_name_path)"`
	RunAttemptID       int64 `xorm:"index unique(runid_attempt_name_path) NOT NULL DEFAULT 0"`
	RunnerID           int64
	RepoID             int64 `xorm:"index"`
	OwnerID            int64
	CommitSHA          string
	StoragePath        string
	FileSize           int64
	FileCompressedSize int64
	ContentEncoding    string             `xorm:"content_encoding"`
	ArtifactPath       string             `xorm:"index unique(runid_attempt_name_path)"`
	ArtifactName       string             `xorm:"index unique(runid_attempt_name_path)"`
	Status             int                `xorm:"index"`
	CreatedUnix        timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix        timeutil.TimeStamp `xorm:"updated index"`
	ExpiredUnix        timeutil.TimeStamp `xorm:"index"`
}

func (actionArtifact) TableName() string {
	return "action_artifact"
}

// actionRun mirrors the post-migration action_run schema.
type actionRun struct {
	ID                int64
	Title             string
	RepoID            int64  `xorm:"unique(repo_index)"`
	OwnerID           int64  `xorm:"index"`
	WorkflowID        string `xorm:"index"`
	Index             int64  `xorm:"index unique(repo_index)"`
	TriggerUserID     int64  `xorm:"index"`
	ScheduleID        int64
	Ref               string `xorm:"index"`
	CommitSHA         string
	IsForkPullRequest bool
	NeedApproval      bool
	ApprovedBy        int64 `xorm:"index"`
	Event             string
	EventPayload      string `xorm:"LONGTEXT"`
	TriggerEvent      string
	Status            int `xorm:"index"`
	Version           int `xorm:"version default 0"`
	RawConcurrency    string
	Started           timeutil.TimeStamp
	Stopped           timeutil.TimeStamp
	PreviousDuration  time.Duration
	LatestAttemptID   int64              `xorm:"index NOT NULL DEFAULT 0"`
	Created           timeutil.TimeStamp `xorm:"created"`
	Updated           timeutil.TimeStamp `xorm:"updated"`
}

func (actionRun) TableName() string {
	return "action_run"
}

// AddActionRunAttemptModel adds the ActionRunAttempt table and the supporting ActionRun/ActionRunJob fields.
func AddActionRunAttemptModel(x *xorm.Engine) error {
	// add "action_run_attempt"
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(actionRunAttempt)); err != nil {
		return err
	}

	// update "action_run_job"
	type ActionRunJob struct {
		RunAttemptID int64 `xorm:"index NOT NULL DEFAULT 0"`
		AttemptJobID int64 `xorm:"index NOT NULL DEFAULT 0"`
		SourceTaskID int64 `xorm:"NOT NULL DEFAULT 0"`
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(ActionRunJob)); err != nil {
		return err
	}

	// update "action_artifact": let xorm sync add the new 4-column unique index (runid_attempt_name_path) and drop the old 3-column unique (runid_name_path)
	if err := x.Sync(new(actionArtifact)); err != nil {
		return err
	}

	// update "action_run"
	//
	// This migration intentionally removes the legacy run-level concurrency columns after
	// introducing attempt-level concurrency on action_run_attempt.
	//
	// Existing values from action_run.concurrency_group / action_run.concurrency_cancel are
	// not backfilled into action_run_attempt:
	//   - the old fields are only meaningful while a run is actively participating in
	//     concurrency scheduling
	//   - for completed legacy runs, keeping or backfilling those values has no practical
	//     effect on future scheduling behavior
	//   - scanning and backfilling old runs would add significant migration cost for little value
	//
	// This means the schema change is destructive for those two legacy columns by design.
	//
	// Let xorm sync add the latest_attempt_id column and drop the now-orphan (repo_id, concurrency_group) index.
	if err := x.Sync(new(actionRun)); err != nil {
		return err
	}
	concurrencyColumns := make([]string, 0, 2)
	for _, col := range []string{"concurrency_group", "concurrency_cancel"} {
		exist, err := x.Dialect().IsColumnExist(x.DB(), context.Background(), "action_run", col)
		if err != nil {
			return err
		}
		if exist {
			concurrencyColumns = append(concurrencyColumns, col)
		}
	}
	if len(concurrencyColumns) == 0 {
		return nil
	}
	sess := x.NewSession()
	defer sess.Close()
	if err := base.DropTableColumns(sess, "action_run", concurrencyColumns...); err != nil {
		return err
	}
	// DropTableColumns rebuilds the table on SQLite, which drops all existing indexes.
	// Re-sync to restore the indexes defined on actionRun.
	return x.Sync(new(actionRun))
}
