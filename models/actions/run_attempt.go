// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"slices"
	"time"

	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/log"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
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

func (attempt *ActionRunAttempt) LoadAttributes(ctx context.Context) (err error) {
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

	return attempt.LoadTriggerUser(ctx)
}

// LoadTriggerUser loads the attempt's trigger user if not already loaded.
func (attempt *ActionRunAttempt) LoadTriggerUser(ctx context.Context) (err error) {
	if attempt.TriggerUser != nil {
		return nil
	}
	attempt.TriggerUserID, attempt.TriggerUser, err = user_model.GetPossibleUserByID(ctx, attempt.TriggerUserID)
	return err
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

// GetArtifactAttemptIDs returns the IDs of the attempts whose artifacts the job may read, newest first,
// always including the job's own attempt.
// An attempt that re-ran only some of the run's jobs keeps the artifacts of the attempt it re-ran from,
// because the jobs it passed through never upload them again; a rerun of the whole run starts over.
func GetArtifactAttemptIDs(ctx context.Context, job *ActionRunJob) ([]int64, error) {
	if job.Attempt <= 1 || job.RunAttemptID == 0 {
		return []int64{job.RunAttemptID}, nil
	}

	attempts, err := ListRunAttemptsByRunID(ctx, job.RunID)
	if err != nil {
		return nil, err
	}
	// a newer attempt is never readable, and attempt 1 has nothing older to continue into
	candidateIDs := container.FilterSlice(attempts, func(a *ActionRunAttempt) (int64, bool) {
		return a.ID, a.Attempt > 1 && a.Attempt <= job.Attempt
	})
	passThroughAttemptIDs, err := findPassThroughAttemptIDs(ctx, candidateIDs)
	if err != nil {
		return nil, err
	}

	ids := make([]int64, 0, len(attempts))
	for _, attempt := range attempts {
		if attempt.Attempt > job.Attempt {
			continue
		}
		ids = append(ids, attempt.ID)
		if !slices.Contains(passThroughAttemptIDs, attempt.ID) {
			// stops at the first attempt that passed no job through
			break
		}
	}
	return ids, nil
}

// findPassThroughAttemptIDs narrows the given attempts to those that were a rerun of selected jobs:
// only such a rerun clones jobs carrying a source task.
// TODO: best-effort. Needs a better way to distinguish between "partial re-run" and "full re-run".
func findPassThroughAttemptIDs(ctx context.Context, attemptIDs []int64) ([]int64, error) {
	passThroughAttemptIDs := make([]int64, 0, len(attemptIDs))
	return passThroughAttemptIDs, db.GetEngine(ctx).
		Table("action_run_job").
		Cols("run_attempt_id").
		In("run_attempt_id", attemptIDs).
		Where("source_task_id <> 0").
		Distinct("run_attempt_id").
		Find(&passThroughAttemptIDs)
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
