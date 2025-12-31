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
	gitvm_ledger "code.gitea.io/gitea/modules/gitvm/ledger"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"github.com/nektos/act/pkg/jobparser"
	"xorm.io/builder"
)

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
	Attempt           int64

	// WorkflowPayload is act/jobparser.SingleWorkflow for act/jobparser.Parse
	// it should contain exactly one job with global workflow fields for this model
	WorkflowPayload []byte

	JobID  string   `xorm:"VARCHAR(255)"` // job id in workflow, not job's id
	Needs  []string `xorm:"JSON TEXT"`
	RunsOn []string `xorm:"JSON TEXT"`
	TaskID int64    // the latest task of the job
	Status Status   `xorm:"index"`

	RawConcurrency string // raw concurrency from job YAML's "concurrency" section

	// IsConcurrencyEvaluated is only valid/needed when this job's RawConcurrency is not empty.
	// If RawConcurrency can't be evaluated (e.g. depend on other job's outputs or have errors), this field will be false.
	// If RawConcurrency has been successfully evaluated, this field will be true, ConcurrencyGroup and ConcurrencyCancel are also set.
	IsConcurrencyEvaluated bool

	ConcurrencyGroup  string `xorm:"index(repo_concurrency) NOT NULL DEFAULT ''"` // evaluated concurrency.group
	ConcurrencyCancel bool   `xorm:"NOT NULL DEFAULT FALSE"`                      // evaluated concurrency.cancel-in-progress

	Started timeutil.TimeStamp
	Stopped timeutil.TimeStamp
	Created timeutil.TimeStamp `xorm:"created"`
	Updated timeutil.TimeStamp `xorm:"updated index"`
}

func init() {
	db.RegisterModel(new(ActionRunJob))
}

func (job *ActionRunJob) Duration() time.Duration {
	return calculateDuration(job.Started, job.Stopped, job.Status)
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

func GetRunJobByID(ctx context.Context, id int64) (*ActionRunJob, error) {
	var job ActionRunJob
	has, err := db.GetEngine(ctx).Where("id=?", id).Get(&job)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("run job with id %d: %w", id, util.ErrNotExist)
	}

	return &job, nil
}

func GetRunJobsByRunID(ctx context.Context, runID int64) (ActionJobList, error) {
	var jobs []*ActionRunJob
	if err := db.GetEngine(ctx).Where("run_id=?", runID).OrderBy("id").Find(&jobs); err != nil {
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
		if job, err = GetRunJobByID(ctx, job.ID); err != nil {
			return 0, err
		}
	}

	{
		// Other goroutines may aggregate the status of the run and update it too.
		// So we need load the run and its jobs before updating the run.
		run, err := GetRunByRepoAndID(ctx, job.RepoID, job.RunID)
		if err != nil {
			return 0, err
		}
		jobs, err := GetRunJobsByRunID(ctx, job.RunID)
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

		// GitVM: emit receipt for CI run end (if terminal state)
		if setting.GitVM.Enabled && run.Status.IsDone() {
			emitCIRunEndReceipt(ctx, run)
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
	runs, jobs, err := GetConcurrentRunsAndJobs(ctx, job.RepoID, job.ConcurrencyGroup, statusFindOption)
	if err != nil {
		return nil, fmt.Errorf("find concurrent runs and jobs: %w", err)
	}
	jobs = slices.DeleteFunc(jobs, func(j *ActionRunJob) bool { return j.ID == job.ID })
	jobsToCancel = append(jobsToCancel, jobs...)

	// cancel runs in the same concurrency group
	for _, run := range runs {
		jobs, err := db.Find[ActionRunJob](ctx, FindRunJobOptions{
			RunID: run.ID,
		})
		if err != nil {
			return nil, fmt.Errorf("find run %d jobs: %w", run.ID, err)
		}
		jobsToCancel = append(jobsToCancel, jobs...)
	}

	return CancelJobs(ctx, jobsToCancel)
}
// emitCIRunEndReceipt emits a GitVM receipt for CI run end (helper to avoid import cycle)
func emitCIRunEndReceipt(ctx context.Context, run *ActionRun) {
	if !setting.GitVM.Enabled {
		return
	}

	// Build receipt inline to avoid import cycle with services/actions
	repoRef := gitvm_ledger.RepoRef{ID: run.RepoID}
	actorRef := gitvm_ledger.ActorRef{ID: run.TriggerUserID}

	// Best-effort lookups
	if run.Repo != nil {
		repoRef.Full = run.Repo.FullName()
	}
	if run.TriggerUser != nil {
		actorRef.Username = run.TriggerUser.Name
	}

	// Compute duration
	durationMs := int64(0)
	if run.Started > 0 && run.Stopped > 0 && run.Stopped >= run.Started {
		durationMs = (int64(run.Stopped) - int64(run.Started)) * 1000
	}

	payload := gitvm_ledger.CIRunEndPayload{
		RunID:      run.ID,
		Status:     run.Status.String(),
		DurationMs: durationMs,
		CommitSHA:  run.CommitSHA,
		WorkflowID: run.WorkflowID,
		Ref:        run.Ref,
		Event:      string(run.Event),
	}

	ts := int64(0)
	if run.Stopped > 0 {
		ts = int64(run.Stopped) * 1000
	}

	receipt := &gitvm_ledger.Receipt{
		Type:     "ci.run.end",
		Repo:     repoRef,
		Actor:    actorRef,
		Payload:  payload,
		TsUnixMs: ts,
	}

	l := gitvm_ledger.New(setting.GitVM.Dir)
	if err := l.Emit(receipt); err != nil {
		log.Warn("GitVM: emit failed (ci.run.end) for run %d: %v", run.ID, err)
	}
}
