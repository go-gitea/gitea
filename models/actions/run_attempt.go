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
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// ActionRunAttempt represents a single execution attempt of an ActionRun.
type ActionRunAttempt struct {
	ID      int64
	RepoID  int64      `xorm:"index"`
	RunID   int64      `xorm:"index UNIQUE(run_attempt)"`
	Run     *ActionRun `xorm:"-"`
	Attempt int64      `xorm:"UNIQUE(run_attempt)"`

	TriggerUserID int64            `xorm:"index"`
	TriggerUser   *user_model.User `xorm:"-"`

	ConcurrencyGroup  string
	ConcurrencyCancel bool `xorm:"NOT NULL DEFAULT FALSE"`

	Status  Status `xorm:"index"`
	Started timeutil.TimeStamp
	Stopped timeutil.TimeStamp

	Created timeutil.TimeStamp `xorm:"created"`
	Updated timeutil.TimeStamp `xorm:"updated"`
}

func (*ActionRunAttempt) TableName() string {
	return "action_run_attempt"
}

func init() {
	db.RegisterModel(new(ActionRunAttempt))
}

func (attempt *ActionRunAttempt) Duration() time.Duration {
	return calculateDuration(attempt.Started, attempt.Stopped, attempt.Status)
}

func GetRunAttemptByRepoAndID(ctx context.Context, repoID, attemptID int64) (*ActionRunAttempt, error) {
	var attempt ActionRunAttempt
	has, err := db.GetEngine(ctx).Where("repo_id=? AND id=?", repoID, attemptID).Get(&attempt)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("run attempt %d in repo %d: %w", attemptID, repoID, util.ErrNotExist)
	}
	return &attempt, nil
}

func GetRunAttemptByRunIDAndAttemptNum(ctx context.Context, runID, attemptNum int64) (*ActionRunAttempt, error) {
	var attempt ActionRunAttempt
	has, err := db.GetEngine(ctx).Where("run_id=? AND attempt=?", runID, attemptNum).Get(&attempt)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("run attempt %d for run %d: %w", attemptNum, runID, util.ErrNotExist)
	}
	return &attempt, nil
}

func ListRunAttemptsByRunID(ctx context.Context, runID int64) ([]*ActionRunAttempt, error) {
	return db.Find[ActionRunAttempt](ctx, &FindRunAttemptOptions{
		RunID:       runID,
		ListOptions: db.ListOptionsAll,
	})
}

func UpdateRunAttempt(ctx context.Context, attempt *ActionRunAttempt, cols ...string) error {
	if slices.Contains(cols, "status") && attempt.Started.IsZero() && attempt.Status.IsRunning() {
		attempt.Started = timeutil.TimeStampNow()
		cols = append(cols, "started")
	}

	sess := db.GetEngine(ctx).ID(attempt.ID)
	if len(cols) > 0 {
		sess.Cols(cols...)
	}
	if _, err := sess.Update(attempt); err != nil {
		return err
	}

	// Only status/timing changes on an attempt need to update the latest run.
	if len(cols) > 0 && !slices.Contains(cols, "status") && !slices.Contains(cols, "started") && !slices.Contains(cols, "stopped") {
		return nil
	}

	run, err := GetRunByRepoAndID(ctx, attempt.RepoID, attempt.RunID)
	if err != nil {
		return err
	}
	if run.LatestAttemptID != attempt.ID {
		log.Warn("run %d cannot be updated by an old attempt %d", run.LatestAttemptID, attempt.ID)
		return nil
	}

	run.Status = attempt.Status
	run.Started = attempt.Started
	run.Stopped = attempt.Stopped
	return UpdateRun(ctx, run, "status", "started", "stopped")
}
