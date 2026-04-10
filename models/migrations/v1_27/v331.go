// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"context"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type actionRunAttempt struct {
	ID                int64
	RepoID            int64 `xorm:"index"`
	RunID             int64 `xorm:"index UNIQUE(run_attempt)"`
	Attempt           int64 `xorm:"UNIQUE(run_attempt)"`
	TriggerUserID     int64 `xorm:"index"`
	Status            int   `xorm:"index"`
	Started           timeutil.TimeStamp
	Stopped           timeutil.TimeStamp
	ConcurrencyGroup  string
	ConcurrencyCancel bool               `xorm:"NOT NULL DEFAULT FALSE"`
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

	// update "action_artifact"
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(actionArtifact)); err != nil {
		return err
	}
	indexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "action_artifact")
	if err != nil {
		return err
	}
	for _, index := range indexes {
		if index.Type == schemas.UniqueType && len(index.Cols) == 3 &&
			index.Cols[0] == "run_id" && index.Cols[1] == "artifact_path" && index.Cols[2] == "artifact_name" {
			if _, err := x.Exec(x.Dialect().DropIndexSQL("action_artifact", index)); err != nil {
				return err
			}
			break
		}
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(actionArtifact)); err != nil {
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
	type ActionRun struct {
		LatestAttemptID int64 `xorm:"index NOT NULL DEFAULT 0"`
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(ActionRun)); err != nil {
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
	_, err = x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(ActionRun))
	return err
}
