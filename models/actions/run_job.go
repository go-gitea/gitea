// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/log"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
	webhook_module "gitea.dev/modules/webhook"

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

	// WorkflowSourceRepoID + WorkflowSourceCommitSHA record the (repo, commit) this job's containing workflow file came from.
	WorkflowSourceRepoID    int64  `xorm:"NOT NULL DEFAULT 0"`
	WorkflowSourceCommitSHA string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`

	// IsReusableCaller marks this job as a reusable workflow caller.
	// Caller jobs do not run on a runner; their status is derived from their child jobs.
	IsReusableCaller bool `xorm:"index NOT NULL DEFAULT FALSE"`
	// IsExpanded reports whether this job's lazy expansion (children-row insertion) is complete.
	// For a reusable workflow caller, true means children rows exist and CallPayload is populated.
	IsExpanded bool `xorm:"NOT NULL DEFAULT FALSE"`
	// CallUses stores the raw "uses:" string of a reusable workflow caller job.
	// Only set when IsReusableCaller is true.
	CallUses string `xorm:"VARCHAR(512) NOT NULL DEFAULT ''"`
	// ReusableWorkflowContent is the content of the reusable workflow specified by "uses:".
	// Only set when IsReusableCaller is true.
	ReusableWorkflowContent []byte `xorm:"LONGBLOB"`
	// CallSecrets encodes the reusable workflow caller's "secrets:" section:
	//   - ""           : no "secrets:" section (children only see auto-generated tokens).
	//   - "inherit"    : the caller wrote "secrets: inherit".
	//   - JSON object  : explicit mapping {alias: source_name}; names only, no values.
	// Only set when IsReusableCaller is true.
	CallSecrets string `xorm:"LONGTEXT"`
	// CallPayload is the JSON-encoded WorkflowCallPayload exposed to children as gitea.event.
	// Populated atomically with IsExpanded at the end of expandReusableWorkflowCaller.
	// Only set when IsReusableCaller is true.
	CallPayload string `xorm:"LONGTEXT"`

	// ParentJobID scopes `Needs` resolution: name lookups happen only among rows sharing the same ParentJobID. 0 for top-level rows.
	ParentJobID int64 `xorm:"index NOT NULL DEFAULT 0"`

	// ContinueOnError mirrors the job-level continue-on-error field from the workflow YAML.
	// When true, a failure of this job does not fail the overall workflow run.
	ContinueOnError bool `xorm:"NOT NULL DEFAULT FALSE"`

	Started timeutil.TimeStamp
	Stopped timeutil.TimeStamp
	Created timeutil.TimeStamp `xorm:"created"`
	Updated timeutil.TimeStamp `xorm:"updated index"`
}

// ActionRunAttemptJobIDIndex backs the run-wide AttemptJobID counter, keyed by ActionRun.ID.
// Use GetNextAttemptJobID to allocate the next ID for a run.
type ActionRunAttemptJobIDIndex db.ResourceIndex

// GetNextAttemptJobID atomically allocates the next AttemptJobID for a job in the given run.
// AttemptJobIDs are unique within a single attempt and stable across attempts for the same logical job
func GetNextAttemptJobID(ctx context.Context, runID int64) (int64, error) {
	return db.GetNextResourceIndex(ctx, "action_run_attempt_job_id_index", runID)
}

func init() {
	db.RegisterModel(new(ActionRunJob))
	db.RegisterModel(new(ActionRunAttemptJobIDIndex))
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

// GetPriorAttemptChildrenByParent returns the children of the most recent prior attempt where
// the parent (identified by parentAttemptJobID) actually had children, indexed by child JobID then child Name.
// Returns (nil, nil) when no such attempt exists.
// The (JobID, Name) key disambiguates both reusable-workflow subtrees and matrix-expanded instances (whose Name carries the matrix suffix).
func GetPriorAttemptChildrenByParent(ctx context.Context, runID, currentAttemptID, parentAttemptJobID int64) (map[string]map[string]*ActionRunJob, error) {
	// query every prior caller row sharing this AttemptJobID, newest first.
	var priorCallers []*ActionRunJob
	if err := db.GetEngine(ctx).
		Where("run_id = ? AND attempt_job_id = ? AND run_attempt_id < ?", runID, parentAttemptJobID, currentAttemptID).
		Desc("run_attempt_id").
		Find(&priorCallers); err != nil {
		return nil, fmt.Errorf("find prior callers: %w", err)
	}
	if len(priorCallers) == 0 {
		return nil, nil //nolint:nilnil // caller is brand new in this attempt
	}

	// query for every child of every prior caller
	callerIDs := make([]int64, len(priorCallers))
	for i, c := range priorCallers {
		callerIDs[i] = c.ID
	}
	var allChildren []*ActionRunJob
	if err := db.GetEngine(ctx).
		Where("run_id = ?", runID).
		In("parent_job_id", callerIDs).
		Find(&allChildren); err != nil {
		return nil, fmt.Errorf("find prior children: %w", err)
	}

	childrenByCallerID := make(map[int64][]*ActionRunJob, len(callerIDs))
	for _, c := range allChildren {
		childrenByCallerID[c.ParentJobID] = append(childrenByCallerID[c.ParentJobID], c)
	}

	// Walk priorCallers in run_attempt_id-desc order and return the children of the first caller that actually had any.
	// Skipped attempts (caller exists but no children) are bypassed.
	for _, caller := range priorCallers {
		children := childrenByCallerID[caller.ID]
		if len(children) == 0 {
			continue
		}
		out := make(map[string]map[string]*ActionRunJob)
		for _, c := range children {
			if out[c.JobID] == nil {
				out[c.JobID] = make(map[string]*ActionRunJob)
			}
			out[c.JobID][c.Name] = c
		}
		return out, nil
	}

	return nil, nil //nolint:nilnil // every prior attempt skipped this caller
}

// GetDirectChildJobsByParent returns the direct child jobs of a parent job (e.g. a reusable workflow caller).
func GetDirectChildJobsByParent(ctx context.Context, parentJob *ActionRunJob) (ActionJobList, error) {
	var jobs []*ActionRunJob
	if err := db.GetEngine(ctx).
		Where("run_id=? AND parent_job_id=?", parentJob.RunID, parentJob.ID).
		OrderBy("id").
		Find(&jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

// CollectAllDescendantJobs returns every job in `allJobs` that lives under parent's subtree (recursively), excluding `parent` itself
func CollectAllDescendantJobs(parent *ActionRunJob, allJobs []*ActionRunJob) []*ActionRunJob {
	parents := map[int64]bool{parent.ID: true}
	for {
		grew := false
		for _, j := range allJobs {
			if j.ParentJobID == 0 {
				continue
			}
			if parents[j.ParentJobID] && !parents[j.ID] {
				parents[j.ID] = true
				grew = true
			}
		}
		if !grew {
			break
		}
	}
	out := make([]*ActionRunJob, 0)
	for _, j := range allJobs {
		if j.ID == parent.ID || !parents[j.ID] {
			continue
		}
		out = append(out, j)
	}
	return out
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

	// xorm's Update writes only non-zero fields when cols is empty, so a zero job.Status
	// with empty cols means status isn't actually being persisted — skip aggregation.
	statusUpdated := slices.Contains(cols, "status") || (len(cols) == 0 && job.Status != 0)
	if affected == 0 || !statusUpdated {
		return affected, nil
	}

	// Reusable workflow caller jobs are never picked up by runners, so they don't need a task-version bump.
	if statusUpdated && job.Status.IsWaiting() && !job.IsReusableCaller {
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

	if statusUpdated && job.ParentJobID > 0 {
		// Reusable workflow caller's children cascade their status changes upward to the parent caller.
		parent, err := GetRunJobByRunAndID(ctx, job.RunID, job.ParentJobID)
		if err != nil {
			return affected, fmt.Errorf("load parent caller %d: %w", job.ParentJobID, err)
		}
		return affected, RefreshReusableCallerStatus(ctx, parent)
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

// RefreshReusableCallerStatus recomputes a reusable workflow caller's Status, Started and Stopped from its current direct children and persists the change.
// No-op if caller is not a reusable caller.
//
// Concurrency: two sibling children finishing at roughly the same time can each invoke this for the same parent caller.
// No row-level lock is taken because AggregateJobStatus is a pure function of the children's statuses (order-independent), so racing callers arrive at the same Status.
func RefreshReusableCallerStatus(ctx context.Context, caller *ActionRunJob) error {
	if !caller.IsReusableCaller {
		return nil
	}
	children, err := GetDirectChildJobsByParent(ctx, caller)
	if err != nil {
		return err
	}

	newStatus := AggregateJobStatus(children)
	cols := make([]string, 0, 3)
	if caller.Status != newStatus {
		caller.Status = newStatus
		cols = append(cols, "status")
	}
	if newStatus != StatusSkipped {
		now := timeutil.TimeStampNow()
		if caller.Started.IsZero() && newStatus == StatusRunning {
			caller.Started = now
			cols = append(cols, "started")
		}
		if caller.Stopped.IsZero() && newStatus.IsDone() {
			caller.Stopped = now
			cols = append(cols, "stopped")
		}
	}
	if len(cols) == 0 {
		return nil
	}
	_, err = UpdateRunJob(ctx, caller, nil, cols...)
	return err
}

func AggregateJobStatus(jobs []*ActionRunJob) Status {
	allSuccessOrSkipped := len(jobs) != 0
	allSkipped := len(jobs) != 0
	var hasFailure, hasCancelled, hasCancelling, hasWaiting, hasRunning, hasBlocked bool
	for _, job := range jobs {
		// A failed job with continue-on-error:true does not fail the workflow run.
		// It counts as a "continued failure" and is treated like success for aggregation.
		isContinuedFailure := job.ContinueOnError && job.Status == StatusFailure
		allSuccessOrSkipped = allSuccessOrSkipped && (job.Status == StatusSuccess || job.Status == StatusSkipped || isContinuedFailure)
		allSkipped = allSkipped && job.Status == StatusSkipped
		hasFailure = hasFailure || (job.Status == StatusFailure && !job.ContinueOnError)
		hasCancelled = hasCancelled || job.Status == StatusCancelled
		hasCancelling = hasCancelling || job.Status == StatusCancelling
		hasWaiting = hasWaiting || job.Status == StatusWaiting
		hasRunning = hasRunning || job.Status == StatusRunning
		hasBlocked = hasBlocked || job.Status == StatusBlocked
	}
	switch {
	case allSkipped:
		return StatusSkipped
	case allSuccessOrSkipped:
		return StatusSuccess
	case hasCancelling:
		return StatusCancelling
	case hasRunning:
		return StatusRunning
	case hasWaiting:
		return StatusWaiting
	case hasBlocked:
		// Blocked is still a pending state, so it should outrank terminal
		// statuses like cancelled/failure when no job is waiting or running.
		return StatusBlocked
	case hasCancelled:
		return StatusCancelled
	case hasFailure:
		return StatusFailure
	default:
		return StatusUnknown // it shouldn't happen
	}
}

// CancelPreviousJobs cancels all previous jobs of the same repository, reference, workflow, and event.
// It's useful when a new run is triggered, and all previous runs needn't be continued anymore.
func CancelPreviousJobs(ctx context.Context, repoID int64, ref, workflowID string, event webhook_module.HookEventType) ([]*ActionRunJob, error) {
	// Find all runs in the specified repository, reference, and workflow with non-final status
	runs, total, err := db.FindAndCount[ActionRun](ctx, FindRunOptions{
		RepoID:       repoID,
		Ref:          ref,
		WorkflowID:   workflowID,
		TriggerEvent: event,
		Status:       []Status{StatusRunning, StatusWaiting, StatusBlocked, StatusCancelling},
	})
	if err != nil {
		return nil, err
	}

	// If there are no runs found, there's no need to proceed with cancellation, so return nil.
	if total == 0 {
		return nil, nil
	}

	cancelledJobs := make([]*ActionRunJob, 0, total)

	// Iterate over each found run and cancel its associated jobs.
	for _, run := range runs {
		// Find all jobs associated with the current run.
		jobs, err := db.Find[ActionRunJob](ctx, FindRunJobOptions{
			RunID: run.ID,
		})
		if err != nil {
			return cancelledJobs, err
		}

		cjs, err := CancelJobs(ctx, jobs)
		if err != nil {
			return cancelledJobs, err
		}
		cancelledJobs = append(cancelledJobs, cjs...)
	}

	// Return nil to indicate successful cancellation of all running and waiting jobs.
	return cancelledJobs, nil
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
		statusFindOption = append(statusFindOption, StatusCancelling)
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

func CancelJobs(ctx context.Context, jobs []*ActionRunJob) ([]*ActionRunJob, error) {
	cancelledJobs := make([]*ActionRunJob, 0, len(jobs))

	for _, job := range jobs {
		if job.IsReusableCaller {
			sub, err := cancelReusableCaller(ctx, job)
			if err != nil {
				return cancelledJobs, err
			}
			cancelledJobs = append(cancelledJobs, sub...)
			continue
		}

		c, err := cancelOneJob(ctx, job)
		if err != nil {
			return cancelledJobs, err
		}
		if c != nil {
			cancelledJobs = append(cancelledJobs, c)
		}
	}
	return cancelledJobs, nil
}

// cancelOneJob cancels a single job and returns the post-cancel row
func cancelOneJob(ctx context.Context, job *ActionRunJob) (*ActionRunJob, error) {
	if job.Status.IsDone() {
		return nil, nil //nolint:nilnil // signal "nothing to cancel; not an error"
	}
	// No associated task: mark Cancelled directly. This includes reusable callers and jobs that never reached PickTask.
	if job.TaskID == 0 {
		job.Status = StatusCancelled
		job.Stopped = timeutil.TimeStampNow()
		n, err := UpdateRunJob(ctx, job, builder.Eq{"task_id": 0}, "status", "stopped")
		if err != nil {
			return nil, err
		}
		if n == 0 {
			log.Error("Failed to cancel job %d because it has changed", job.ID)
			return nil, nil //nolint:nilnil // signal "nothing to cancel; not an error"
		}
		return job, nil
	}
	// Has a task: stop the task and re-read the row.
	if err := StopTask(ctx, job.TaskID, StatusCancelling); err != nil {
		return nil, err
	}
	updated, err := GetRunJobByRunAndID(ctx, job.RunID, job.ID)
	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}
	return updated, nil
}

// cancelReusableCaller cancels `caller` and all its child jobs
func cancelReusableCaller(ctx context.Context, caller *ActionRunJob) ([]*ActionRunJob, error) {
	cancelledJobs := make([]*ActionRunJob, 0)

	attemptJobs, err := GetRunJobsByRunAndAttemptID(ctx, caller.RunID, caller.RunAttemptID)
	if err != nil {
		return cancelledJobs, err
	}

	// Cancel descendants deepest-first, then the caller: a caller's status is aggregated from its children,
	// so each child must reach its final state before its parent caller is re-aggregated.
	// A child's ID always exceeds its parent's, so descending ID is a valid deepest-first order.
	descendants := CollectAllDescendantJobs(caller, attemptJobs)
	slices.SortFunc(descendants, func(a, b *ActionRunJob) int { return cmp.Compare(b.ID, a.ID) })

	for _, c := range descendants {
		cancelled, err := cancelOneJob(ctx, c)
		if err != nil {
			return cancelledJobs, err
		}
		if cancelled != nil {
			cancelledJobs = append(cancelledJobs, cancelled)
		}
	}

	if c, err := cancelOneJob(ctx, caller); err != nil {
		return cancelledJobs, err
	} else if c != nil {
		cancelledJobs = append(cancelledJobs, c)
	}
	return cancelledJobs, nil
}
