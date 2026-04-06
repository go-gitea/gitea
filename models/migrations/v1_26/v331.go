// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"context"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
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
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(actionRunAttempt)); err != nil {
		return err
	}

	type ActionRun struct {
		LatestAttemptID int64 `xorm:"index NOT NULL DEFAULT 0"`
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(ActionRun)); err != nil {
		return err
	}

	type ActionRunJob struct {
		RunAttemptID int64 `xorm:"index NOT NULL DEFAULT 0"`
		SourceTaskID int64 `xorm:"NOT NULL DEFAULT 0"`
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(ActionRunJob)); err != nil {
		return err
	}

	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(actionArtifact)); err != nil {
		return err
	}

	if err := base.RecreateTables(new(actionArtifact))(x); err != nil {
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
	return base.DropTableColumns(sess, "action_run", concurrencyColumns...)
}
