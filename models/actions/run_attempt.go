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
	RepoID  int64      `xorm:"index(repo_concurrency_status)"`
	RunID   int64      `xorm:"UNIQUE(run_attempt)"`
	Run     *ActionRun `xorm:"-"`
	Attempt int64      `xorm:"UNIQUE(run_attempt)"`

	TriggerUserID int64
	TriggerUser   *user_model.User `xorm:"-"`

	ConcurrencyGroup  string `xorm:"index(repo_concurrency_status) NOT NULL DEFAULT ''"`
	ConcurrencyCancel bool   `xorm:"NOT NULL DEFAULT FALSE"`

	Status  Status `xorm:"index(repo_concurrency_status)"`
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
	return calculateDuration(attempt.Started, attempt.Stopped, attempt.Status, attempt.Updated)
}

func (attempt *ActionRunAttempt) LoadAttributes(ctx context.Context) error {
	if attempt == nil {
		return nil
	}

	if attempt.Run == nil {
		run, err := GetRunByRepoAndID(ctx, attempt.RepoID, attempt.RunID)
		if err != nil {
			return err
		}
		if err := run.LoadAttributes(ctx); err != nil {
			return err
		}
		attempt.Run = run
	}

	if attempt.TriggerUser == nil {
		u, err := user_model.GetPossibleUserByID(ctx, attempt.TriggerUserID)
		if err != nil {
			return err
		}
		attempt.TriggerUser = u
	}

	return nil
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

// FindConcurrentRunAttempts returns attempts in the given concurrency group and status set.
// Results are unordered; callers must not depend on any particular row order.
func FindConcurrentRunAttempts(ctx context.Context, repoID int64, concurrencyGroup string, statuses []Status) ([]*ActionRunAttempt, error) {
	attempts := make([]*ActionRunAttempt, 0)
	sess := db.GetEngine(ctx).Where("repo_id=? AND concurrency_group=?", repoID, concurrencyGroup)
	if len(statuses) > 0 {
		sess = sess.In("status", statuses)
	}
	return attempts, sess.Find(&attempts)
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
