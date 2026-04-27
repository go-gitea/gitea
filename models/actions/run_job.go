// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"slices"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/actions/jobparser"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// MaxJobNumPerRun is the maximum number of jobs in a single run.
// https://docs.github.com/en/actions/reference/limits#existing-system-limits
// TODO: check this limit when creating jobs
const MaxJobNumPerRun = 256

// ActionRunJob represents a job of a run
type ActionRunJob struct {
	ID                int64
	RunID             int64                  `xorm:"index"`
	Run               *ActionRun             `xorm:"-"`
	RepoID            int64                  `xorm:"index(repo_concurrency)"`
	Repo              *repo_model.Repository `xorm:"-"`
	OwnerID           int64                  `xorm:"index"`
	CommitSHA         string                 `xorm:"index"`
	IsForkPullRequest bool
	Name              string `xorm:"VARCHAR(255)"`

	// for legacy jobs, this counts how many times the job has run;
	// otherwise it matches the Attempt of the ActionRunAttempt identified by job.RunAttemptID
	Attempt int64

	// WorkflowPayload is act/jobparser.SingleWorkflow for act/jobparser.Parse
	// it should contain exactly one job with global workflow fields for this model
	WorkflowPayload []byte

	JobID  string   `xorm:"VARCHAR(255)"` // job id in workflow, not job's id
	Needs  []string `xorm:"JSON TEXT"`
	RunsOn []string `xorm:"JSON TEXT"`

	TaskID       int64 // the task created by this job in its own attempt
	SourceTaskID int64 `xorm:"NOT NULL DEFAULT 0"` // SourceTaskID points to a historical task when this job reuses an earlier attempt's result.

	Status Status `xorm:"index"`

	RawConcurrency string // raw concurrency from job YAML's "concurrency" section

	// IsConcurrencyEvaluated is only valid/needed when this job's RawConcurrency is not empty.
	// If RawConcurrency can't be evaluated (e.g. depend on other job's outputs or have errors), this field will be false.
	// If RawConcurrency has been successfully evaluated, this field will be true, ConcurrencyGroup and ConcurrencyCancel are also set.
	IsConcurrencyEvaluated bool

	ConcurrencyGroup  string `xorm:"index(repo_concurrency) NOT NULL DEFAULT ''"` // evaluated concurrency.group
	ConcurrencyCancel bool   `xorm:"NOT NULL DEFAULT FALSE"`                      // evaluated concurrency.cancel-in-progress

	// TokenPermissions stores the explicit permissions from workflow/job YAML (no org/repo clamps applied).
	// Org/repo clamps are enforced when the token is used at runtime.
	// It is JSON-encoded repo_model.ActionsTokenPermissions and may be empty if not specified.
	TokenPermissions *repo_model.ActionsTokenPermissions `xorm:"JSON TEXT"`

	// RunAttemptID identifies the ActionRunAttempt this job belongs to.
	// A value of 0 indicates a legacy job created before ActionRunAttempt existed.
	RunAttemptID int64 `xorm:"index NOT NULL DEFAULT 0"`
	// AttemptJobID is unique within a single attempt.
	// For jobs created after ActionRunAttempt was introduced, the same logical job is expected to keep the same AttemptJobID across attempts.
	// A value of 0 indicates a legacy job created before ActionRunAttempt existed.
	AttemptJobID int64 `xorm:"index NOT NULL DEFAULT 0"`

	Started timeutil.TimeStamp
	Stopped timeutil.TimeStamp
	Created timeutil.TimeStamp `xorm:"created"`
	Updated timeutil.TimeStamp `xorm:"updated index"`
}

func init() {
	db.RegisterModel(new(ActionRunJob))
}

func (job *ActionRunJob) Duration() time.Duration {
	return calculateDuration(job.Started, job.Stopped, job.Status, job.Updated)
}

func (job *ActionRunJob) EffectiveTaskID() int64 {
	if job.TaskID > 0 {
		return job.TaskID
	}
	return job.SourceTaskID
}

func (job *ActionRunJob) LoadRun(ctx context.Context) error {
	if job.Run == nil {
		run, err := GetRunByRepoAndID(ctx, job.RepoID, job.RunID)
		if err != nil {
			return err
		}
		job.Run = run
	}
	return nil
}

func (job *ActionRunJob) LoadRepo(ctx context.Context) error {
	if job.Repo == nil {
		repo, err := repo_model.GetRepositoryByID(ctx, job.RepoID)
		if err != nil {
			return err
		}
		job.Repo = repo
	}
	return nil
}

// LoadAttributes load Run if not loaded
func (job *ActionRunJob) LoadAttributes(ctx context.Context) error {
	if job == nil {
		return nil
	}

	if err := job.LoadRun(ctx); err != nil {
		return err
	}

	return job.Run.LoadAttributes(ctx)
}

// ParseJob parses the job structure from the ActionRunJob.WorkflowPayload
func (job *ActionRunJob) ParseJob() (*jobparser.Job, error) {
	// job.WorkflowPayload is a SingleWorkflow created from an ActionRun's workflow, which exactly contains this job's YAML definition.
	// Ideally it shouldn't be called "Workflow", it is just a job with global workflow fields + trigger
	parsedWorkflows, err := jobparser.Parse(job.WorkflowPayload)
	if err != nil {
		return nil, fmt.Errorf("job %d single workflow: unable to parse: %w", job.ID, err)
	} else if len(parsedWorkflows) != 1 {
		return nil, fmt.Errorf("job %d single workflow: not single workflow", job.ID)
	}
	_, workflowJob := parsedWorkflows[0].Job()
	if workflowJob == nil {
		// it shouldn't happen, and since the callers don't check nil, so return an error instead of nil
		return nil, util.ErrorWrap(util.ErrNotExist, "job %d single workflow: payload doesn't contain a job", job.ID)
	}
	return workflowJob, nil
}

func GetRunJobByRepoAndID(ctx context.Context, repoID, jobID int64) (*ActionRunJob, error) {
	var job ActionRunJob
	has, err := db.GetEngine(ctx).Where("id=? AND repo_id=?", jobID, repoID).Get(&job)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("run job with id %d: %w", jobID, util.ErrNotExist)
	}

	return &job, nil
}

func GetRunJobByRunAndID(ctx context.Context, runID, jobID int64) (*ActionRunJob, error) {
	var job ActionRunJob
	has, err := db.GetEngine(ctx).Where("id=? AND run_id=?", jobID, runID).Get(&job)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("run job with id %d: %w", jobID, util.ErrNotExist)
	}

	return &job, nil
}

func GetRunJobByAttemptJobID(ctx context.Context, runID, attemptID, attemptJobID int64) (*ActionRunJob, error) {
	var job ActionRunJob
	has, err := db.GetEngine(ctx).Where("run_id=? AND run_attempt_id=? AND attempt_job_id=?", runID, attemptID, attemptJobID).Get(&job)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("run job with attempt_job_id %d in run %d attempt %d: %w", attemptJobID, runID, attemptID, util.ErrNotExist)
	}

	return &job, nil
}

// GetLatestAttemptJobsByRepoAndRunID returns the jobs of the latest attempt for a run.
// It prefers the latest attempt when one exists, and falls back to legacy jobs with run_attempt_id=0 for runs created before ActionRunAttempt existed.
func GetLatestAttemptJobsByRepoAndRunID(ctx context.Context, repoID, runID int64) (ActionJobList, error) {
	run, err := GetRunByRepoAndID(ctx, repoID, runID)
	if err != nil {
		return nil, err
	}
	if run.LatestAttemptID > 0 {
		return GetRunJobsByRunAndAttemptID(ctx, runID, run.LatestAttemptID)
	}

	var jobs []*ActionRunJob
	if err := db.GetEngine(ctx).Where("repo_id=? AND run_id=? AND run_attempt_id=0", repoID, runID).OrderBy("id").Find(&jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

// GetAllRunJobsByRepoAndRunID returns all jobs for a run across all attempts.
func GetAllRunJobsByRepoAndRunID(ctx context.Context, repoID, runID int64) (ActionJobList, error) {
	var jobs []*ActionRunJob
	if err := db.GetEngine(ctx).Where("repo_id=? AND run_id=?", repoID, runID).OrderBy("id").Find(&jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

// GetRunJobsByRunAndAttemptID returns jobs for a run within a specific attempt.
// runAttemptID may be 0 to address legacy jobs that were created before ActionRunAttempt existed and therefore have no attempt association.
func GetRunJobsByRunAndAttemptID(ctx context.Context, runID, runAttemptID int64) (ActionJobList, error) {
	var jobs []*ActionRunJob
	if err := db.GetEngine(ctx).Where("run_id=? AND run_attempt_id=?", runID, runAttemptID).OrderBy("id").Find(&jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func UpdateRunJob(ctx context.Context, job *ActionRunJob, cond builder.Cond, cols ...string) (int64, error) {
	e := db.GetEngine(ctx)

	sess := e.ID(job.ID)
	if len(cols) > 0 {
		sess.Cols(cols...)
	}

	if cond != nil {
		sess.Where(cond)
	}

	affected, err := sess.Update(job)
	if err != nil {
		return 0, err
	}

	if affected == 0 || (!slices.Contains(cols, "status") && job.Status == 0) {
		return affected, nil
	}

	if slices.Contains(cols, "status") && job.Status.IsWaiting() {
		// if the status of job changes to waiting again, increase tasks version.
		if err := IncreaseTaskVersion(ctx, job.OwnerID, job.RepoID); err != nil {
			return 0, err
		}
	}

	if job.RunID == 0 {
		var err error
		if job, err = GetRunJobByRepoAndID(ctx, job.RepoID, job.ID); err != nil {
			return 0, err
		}
	}

	{
		// Other goroutines may aggregate the status of the attempt/run and update it too.
		// So we need to load the current jobs before updating the aggregate state.
		if job.RunAttemptID > 0 {
			attempt, err := GetRunAttemptByRepoAndID(ctx, job.RepoID, job.RunAttemptID)
			if err != nil {
				return 0, err
			}
			jobs, err := GetRunJobsByRunAndAttemptID(ctx, job.RunID, job.RunAttemptID)
			if err != nil {
				return 0, err
			}
			attempt.Status = AggregateJobStatus(jobs)
			if attempt.Started.IsZero() && attempt.Status.IsRunning() {
				attempt.Started = timeutil.TimeStampNow()
			}
			if attempt.Stopped.IsZero() && attempt.Status.IsDone() {
				attempt.Stopped = timeutil.TimeStampNow()
			}
			if err := UpdateRunAttempt(ctx, attempt, "status", "started", "stopped"); err != nil {
				return 0, fmt.Errorf("update run attempt %d: %w", attempt.ID, err)
			}
		} else {
			// TODO: Remove this fallback in the future.
			// Legacy fallback: jobs created before migration v331 have RunAttemptID=0 and are NOT backfilled.
			// This path keeps those runs' status consistent when their jobs finish, including:
			//   - jobs created before migration v331 and complete on the new version starts
			//   - zombie/abandoned cleanup cron tasks that call UpdateRunJob on legacy jobs
			run, err := GetRunByRepoAndID(ctx, job.RepoID, job.RunID)
			if err != nil {
				return 0, err
			}
			jobs, err := GetLatestAttemptJobsByRepoAndRunID(ctx, job.RepoID, job.RunID)
			if err != nil {
				return 0, err
			}
			run.Status = AggregateJobStatus(jobs)
			if run.Started.IsZero() && run.Status.IsRunning() {
				run.Started = timeutil.TimeStampNow()
			}
			if run.Stopped.IsZero() && run.Status.IsDone() {
				run.Stopped = timeutil.TimeStampNow()
			}
			if err := UpdateRun(ctx, run, "status", "started", "stopped"); err != nil {
				return 0, fmt.Errorf("update run %d: %w", run.ID, err)
			}
		}
	}

	return affected, nil
}

func AggregateJobStatus(jobs []*ActionRunJob) Status {
	allSuccessOrSkipped := len(jobs) != 0
	allSkipped := len(jobs) != 0
	var hasFailure, hasCancelled, hasWaiting, hasRunning, hasBlocked bool
	for _, job := range jobs {
		allSuccessOrSkipped = allSuccessOrSkipped && (job.Status == StatusSuccess || job.Status == StatusSkipped)
		allSkipped = allSkipped && job.Status == StatusSkipped
		hasFailure = hasFailure || job.Status == StatusFailure
		hasCancelled = hasCancelled || job.Status == StatusCancelled
		hasWaiting = hasWaiting || job.Status == StatusWaiting
		hasRunning = hasRunning || job.Status == StatusRunning
		hasBlocked = hasBlocked || job.Status == StatusBlocked
	}
	switch {
	case allSkipped:
		return StatusSkipped
	case allSuccessOrSkipped:
		return StatusSuccess
	case hasCancelled:
		return StatusCancelled
	case hasRunning:
		return StatusRunning
	case hasWaiting:
		return StatusWaiting
	case hasFailure:
		return StatusFailure
	case hasBlocked:
		return StatusBlocked
	default:
		return StatusUnknown // it shouldn't happen
	}
}

func CancelPreviousJobsByJobConcurrency(ctx context.Context, job *ActionRunJob) (jobsToCancel []*ActionRunJob, _ error) {
	if job.RawConcurrency == "" {
		return nil, nil
	}
	if !job.IsConcurrencyEvaluated {
		return nil, nil
	}
	if job.ConcurrencyGroup == "" {
		return nil, nil
	}

	statusFindOption := []Status{StatusWaiting, StatusBlocked}
	if job.ConcurrencyCancel {
		statusFindOption = append(statusFindOption, StatusRunning)
	}
	attempts, jobs, err := GetConcurrentRunAttemptsAndJobs(ctx, job.RepoID, job.ConcurrencyGroup, statusFindOption)
	if err != nil {
		return nil, fmt.Errorf("find concurrent runs and jobs: %w", err)
	}
	jobs = slices.DeleteFunc(jobs, func(j *ActionRunJob) bool { return j.ID == job.ID })
	jobsToCancel = append(jobsToCancel, jobs...)

	// cancel runs in the same concurrency group
	for _, attempt := range attempts {
		if attempt.ID == job.RunAttemptID {
			continue
		}
		jobs, err := GetRunJobsByRunAndAttemptID(ctx, attempt.RunID, attempt.ID)
		if err != nil {
			return nil, fmt.Errorf("find run %d attempt %d jobs: %w", attempt.RunID, attempt.ID, err)
		}
		jobsToCancel = append(jobsToCancel, jobs...)
	}

	return CancelJobs(ctx, jobsToCancel)
}
