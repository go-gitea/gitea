// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"slices"
	"time"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// RunAttempt represents a single execution attempt of an ActionRun.
type RunAttempt struct {
	ID      int64
	RepoID  int64      `xorm:"index"`
	RunID   int64      `xorm:"index UNIQUE(run_attempt)"`
	Run     *ActionRun `xorm:"-"`
	Attempt int64      `xorm:"UNIQUE(run_attempt)"`

	TriggerUserID int64            `xorm:"index"`
	TriggerUser   *user_model.User `xorm:"-"`

	ConcurrencyGroup  string
	ConcurrencyCancel bool `xorm:"NOT NULL DEFAULT FALSE"`

	Status       Status `xorm:"index"`
	Started      timeutil.TimeStamp
	RunStartedAt timeutil.TimeStamp
	Stopped      timeutil.TimeStamp

	Created timeutil.TimeStamp `xorm:"created"`
	Updated timeutil.TimeStamp `xorm:"updated"`
}

func (*RunAttempt) TableName() string {
	return "action_run_attempt"
}

func init() {
	db.RegisterModel(new(RunAttempt))
}

func (attempt *RunAttempt) Duration() time.Duration {
	return calculateDuration(attempt.Started, attempt.Stopped, attempt.Status)
}

func GetRunAttemptByRepoAndID(ctx context.Context, repoID, attemptID int64) (*RunAttempt, error) {
	var attempt RunAttempt
	has, err := db.GetEngine(ctx).Where("repo_id=? AND id=?", repoID, attemptID).Get(&attempt)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("run attempt %d in repo %d: %w", attemptID, repoID, util.ErrNotExist)
	}
	return &attempt, nil
}

func GetLatestAttemptByRunID(ctx context.Context, runID int64) (*RunAttempt, bool, error) {
	var attempt RunAttempt
	has, err := db.GetEngine(ctx).Where("run_id=?", runID).Desc("attempt").Get(&attempt)
	if err != nil {
		return nil, false, err
	} else if !has {
		return nil, false, nil
	}
	return &attempt, true, nil
}

func GetRunAttemptByRunIDAndAttemptNum(ctx context.Context, runID, attemptNum int64) (*RunAttempt, error) {
	var attempt RunAttempt
	has, err := db.GetEngine(ctx).Where("run_id=? AND attempt=?", runID, attemptNum).Get(&attempt)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("run attempt %d for run %d: %w", attemptNum, runID, util.ErrNotExist)
	}
	return &attempt, nil
}

func ListRunAttemptsByRunID(ctx context.Context, runID int64) ([]*RunAttempt, error) {
	return db.Find[RunAttempt](ctx, &FindRunAttemptOptions{
		RunID:       runID,
		ListOptions: db.ListOptionsAll,
	})
}

func UpdateRunAttempt(ctx context.Context, attempt *RunAttempt, cols ...string) error {
	sess := db.GetEngine(ctx).ID(attempt.ID)
	if len(cols) > 0 {
		sess.Cols(cols...)
	}
	if _, err := sess.Update(attempt); err != nil {
		return err
	}

	if len(cols) > 0 && !slices.Contains(cols, "status") && !slices.Contains(cols, "started") && !slices.Contains(cols, "stopped") {
		return nil
	}

	run, err := GetRunByRepoAndID(ctx, attempt.RepoID, attempt.RunID)
	if err != nil {
		return err
	}
	if run.LatestAttemptID != attempt.ID {
		return nil
	}

	run.Status = attempt.Status
	run.Started = attempt.Started
	run.Stopped = attempt.Stopped
	return UpdateRun(ctx, run, "status", "started", "stopped")
}
