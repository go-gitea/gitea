// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"xorm.io/builder"
)

// ActionRun represents a run of a workflow file
type ActionRun struct {
	ID                int64
	Title             string
	RepoID            int64                  `xorm:"unique(repo_index) index(repo_concurrency)"`
	Repo              *repo_model.Repository `xorm:"-"`
	OwnerID           int64                  `xorm:"index"`
	WorkflowID        string                 `xorm:"index"`                    // the name of workflow file
	Index             int64                  `xorm:"index unique(repo_index)"` // a unique number for each run of a repository
	TriggerUserID     int64                  `xorm:"index"`
	TriggerUser       *user_model.User       `xorm:"-"`
	ScheduleID        int64
	Ref               string `xorm:"index"` // the commit/tag/… that caused the run
	IsRefDeleted      bool   `xorm:"-"`
	CommitSHA         string
	IsForkPullRequest bool                         // If this is triggered by a PR from a forked repository or an untrusted user, we need to check if it is approved and limit permissions when running the workflow.
	NeedApproval      bool                         // may need approval if it's a fork pull request
	ApprovedBy        int64                        `xorm:"index"` // who approved
	Event             webhook_module.HookEventType // the webhook event that causes the workflow to run
	EventPayload      string                       `xorm:"LONGTEXT"`
	TriggerEvent      string                       // the trigger event defined in the `on` configuration of the triggered workflow
	Status            Status                       `xorm:"index"`
	Version           int                          `xorm:"version default 0"` // Status could be updated concomitantly, so an optimistic lock is needed
	RawConcurrency    string                       // raw concurrency
	ConcurrencyGroup  string                       `xorm:"index(repo_concurrency) NOT NULL DEFAULT ''"`
	ConcurrencyCancel bool                         `xorm:"NOT NULL DEFAULT FALSE"`
	// Started and Stopped is used for recording last run time, if rerun happened, they will be reset to 0
	Started timeutil.TimeStamp
	Stopped timeutil.TimeStamp
	// PreviousDuration is used for recording previous duration
	PreviousDuration time.Duration
	Created          timeutil.TimeStamp `xorm:"created"`
	Updated          timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ActionRun))
	db.RegisterModel(new(ActionRunIndex))
}

func (run *ActionRun) HTMLURL() string {
	if run.Repo == nil {
		return ""
	}
	return fmt.Sprintf("%s/actions/runs/%d", run.Repo.HTMLURL(), run.Index)
}

func (run *ActionRun) Link() string {
	if run.Repo == nil {
		return ""
	}
	return fmt.Sprintf("%s/actions/runs/%d", run.Repo.Link(), run.Index)
}

func (run *ActionRun) WorkflowLink() string {
	if run.Repo == nil {
		return ""
	}
	return fmt.Sprintf("%s/actions/?workflow=%s", run.Repo.Link(), run.WorkflowID)
}

// RefLink return the url of run's ref
func (run *ActionRun) RefLink() string {
	refName := git.RefName(run.Ref)
	if refName.IsPull() {
		return run.Repo.Link() + "/pulls/" + refName.ShortName()
	}
	return run.Repo.Link() + "/src/" + refName.RefWebLinkPath()
}

// PrettyRef return #id for pull ref or ShortName for others
func (run *ActionRun) PrettyRef() string {
	refName := git.RefName(run.Ref)
	if refName.IsPull() {
		return "#" + strings.TrimSuffix(strings.TrimPrefix(run.Ref, git.PullPrefix), "/head")
	}
	return refName.ShortName()
}

// RefTooltip return a tooltop of run's ref. For pull request, it's the title of the PR, otherwise it's the ShortName.
func (run *ActionRun) RefTooltip() string {
	payload, err := run.GetPullRequestEventPayload()
	if err == nil && payload != nil && payload.PullRequest != nil {
		return payload.PullRequest.Title
	}
	return git.RefName(run.Ref).ShortName()
}

// LoadAttributes load Repo TriggerUser if not loaded
func (run *ActionRun) LoadAttributes(ctx context.Context) error {
	if run == nil {
		return nil
	}

	if err := run.LoadRepo(ctx); err != nil {
		return err
	}

	if err := run.Repo.LoadAttributes(ctx); err != nil {
		return err
	}

	if run.TriggerUser == nil {
		u, err := user_model.GetPossibleUserByID(ctx, run.TriggerUserID)
		if err != nil {
			return err
		}
		run.TriggerUser = u
	}

	return nil
}

func (run *ActionRun) LoadRepo(ctx context.Context) error {
	if run == nil || run.Repo != nil {
		return nil
	}

	repo, err := repo_model.GetRepositoryByID(ctx, run.RepoID)
	if err != nil {
		return err
	}
	run.Repo = repo
	return nil
}

func (run *ActionRun) Duration() time.Duration {
	return calculateDuration(run.Started, run.Stopped, run.Status) + run.PreviousDuration
}

func (run *ActionRun) GetPushEventPayload() (*api.PushPayload, error) {
	if run.Event == webhook_module.HookEventPush {
		var payload api.PushPayload
		if err := json.Unmarshal([]byte(run.EventPayload), &payload); err != nil {
			return nil, err
		}
		return &payload, nil
	}
	return nil, fmt.Errorf("event %s is not a push event", run.Event)
}

func (run *ActionRun) GetPullRequestEventPayload() (*api.PullRequestPayload, error) {
	if run.Event.IsPullRequest() {
		var payload api.PullRequestPayload
		if err := json.Unmarshal([]byte(run.EventPayload), &payload); err != nil {
			return nil, err
		}
		return &payload, nil
	}
	return nil, fmt.Errorf("event %s is not a pull request event", run.Event)
}

func (run *ActionRun) GetWorkflowRunEventPayload() (*api.WorkflowRunPayload, error) {
	if run.Event == webhook_module.HookEventWorkflowRun {
		var payload api.WorkflowRunPayload
		if err := json.Unmarshal([]byte(run.EventPayload), &payload); err != nil {
			return nil, err
		}
		return &payload, nil
	}
	return nil, fmt.Errorf("event %s is not a workflow run event", run.Event)
}

func (run *ActionRun) IsSchedule() bool {
	return run.ScheduleID > 0
}

// UpdateRepoRunsNumbers updates the number of runs and closed runs of a repository.
func UpdateRepoRunsNumbers(ctx context.Context, repo *repo_model.Repository) error {
	_, err := db.GetEngine(ctx).ID(repo.ID).
		NoAutoTime().
		Cols("num_action_runs", "num_closed_action_runs").
		SetExpr("num_action_runs",
			builder.Select("count(*)").From("action_run").
				Where(builder.Eq{"repo_id": repo.ID}),
		).
		SetExpr("num_closed_action_runs",
			builder.Select("count(*)").From("action_run").
				Where(builder.Eq{
					"repo_id": repo.ID,
				}.And(
					builder.In("status",
						StatusSuccess,
						StatusFailure,
						StatusCancelled,
						StatusSkipped,
					),
				),
				),
		).
		Update(repo)
	return err
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
		Status:       []Status{StatusRunning, StatusWaiting, StatusBlocked},
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

func CancelJobs(ctx context.Context, jobs []*ActionRunJob) ([]*ActionRunJob, error) {
	cancelledJobs := make([]*ActionRunJob, 0, len(jobs))
	// Iterate over each job and attempt to cancel it.
	for _, job := range jobs {
		// Skip jobs that are already in a terminal state (completed, cancelled, etc.).
		status := job.Status
		if status.IsDone() {
			continue
		}

		// If the job has no associated task (probably an error), set its status to 'Cancelled' and stop it.
		if job.TaskID == 0 {
			job.Status = StatusCancelled
			job.Stopped = timeutil.TimeStampNow()

			// Update the job's status and stopped time in the database.
			n, err := UpdateRunJob(ctx, job, builder.Eq{"task_id": 0}, "status", "stopped")
			if err != nil {
				return cancelledJobs, err
			}

			// If the update affected 0 rows, it means the job has changed in the meantime
			if n == 0 {
				log.Error("Failed to cancel job %d because it has changed", job.ID)
				continue
			}

			cancelledJobs = append(cancelledJobs, job)
			// Continue with the next job.
			continue
		}

		// If the job has an associated task, try to stop the task, effectively cancelling the job.
		if err := StopTask(ctx, job.TaskID, StatusCancelled); err != nil {
			return cancelledJobs, err
		}
		updatedJob, err := GetRunJobByID(ctx, job.ID)
		if err != nil {
			return cancelledJobs, fmt.Errorf("get job: %w", err)
		}
		cancelledJobs = append(cancelledJobs, updatedJob)
	}

	// Return nil to indicate successful cancellation of all running and waiting jobs.
	return cancelledJobs, nil
}

func GetRunByRepoAndID(ctx context.Context, repoID, runID int64) (*ActionRun, error) {
	var run ActionRun
	has, err := db.GetEngine(ctx).Where("id=? AND repo_id=?", runID, repoID).Get(&run)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("run with id %d: %w", runID, util.ErrNotExist)
	}

	return &run, nil
}

func GetRunByIndex(ctx context.Context, repoID, index int64) (*ActionRun, error) {
	run := &ActionRun{
		RepoID: repoID,
		Index:  index,
	}
	has, err := db.GetEngine(ctx).Get(run)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("run with index %d %d: %w", repoID, index, util.ErrNotExist)
	}

	return run, nil
}

func GetLatestRun(ctx context.Context, repoID int64) (*ActionRun, error) {
	run := &ActionRun{
		RepoID: repoID,
	}
	has, err := db.GetEngine(ctx).Where("repo_id=?", repoID).Desc("index").Get(run)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("latest run with repo_id %d: %w", repoID, util.ErrNotExist)
	}
	return run, nil
}

func GetWorkflowLatestRun(ctx context.Context, repoID int64, workflowFile, branch, event string) (*ActionRun, error) {
	var run ActionRun
	q := db.GetEngine(ctx).Where("repo_id=?", repoID).
		And("ref = ?", branch).
		And("workflow_id = ?", workflowFile)
	if event != "" {
		q.And("event = ?", event)
	}
	has, err := q.Desc("id").Get(&run)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, util.NewNotExistErrorf("run with repo_id %d, ref %s, workflow_id %s", repoID, branch, workflowFile)
	}
	return &run, nil
}

// UpdateRun updates a run.
// It requires the inputted run has Version set.
// It will return error if the version is not matched (it means the run has been changed after loaded).
func UpdateRun(ctx context.Context, run *ActionRun, cols ...string) error {
	sess := db.GetEngine(ctx).ID(run.ID)
	if len(cols) > 0 {
		sess.Cols(cols...)
	}
	run.Title = util.EllipsisDisplayString(run.Title, 255)
	affected, err := sess.Update(run)
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("run has changed")
		// It's impossible that the run is not found, since Gitea never deletes runs.
	}

	if run.Status != 0 || slices.Contains(cols, "status") {
		if run.RepoID == 0 {
			setting.PanicInDevOrTesting("RepoID should not be 0")
		}
		if err = run.LoadRepo(ctx); err != nil {
			return err
		}
		if err := UpdateRepoRunsNumbers(ctx, run.Repo); err != nil {
			return err
		}
	}

	return nil
}

type ActionRunIndex db.ResourceIndex

func GetConcurrentRunsAndJobs(ctx context.Context, repoID int64, concurrencyGroup string, status []Status) ([]*ActionRun, []*ActionRunJob, error) {
	runs, err := db.Find[ActionRun](ctx, &FindRunOptions{
		RepoID:           repoID,
		ConcurrencyGroup: concurrencyGroup,
		Status:           status,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("find runs: %w", err)
	}

	jobs, err := db.Find[ActionRunJob](ctx, &FindRunJobOptions{
		RepoID:           repoID,
		ConcurrencyGroup: concurrencyGroup,
		Statuses:         status,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("find jobs: %w", err)
	}

	return runs, jobs, nil
}

func CancelPreviousJobsByRunConcurrency(ctx context.Context, actionRun *ActionRun) ([]*ActionRunJob, error) {
	if actionRun.ConcurrencyGroup == "" {
		return nil, nil
	}

	var jobsToCancel []*ActionRunJob

	statusFindOption := []Status{StatusWaiting, StatusBlocked}
	if actionRun.ConcurrencyCancel {
		statusFindOption = append(statusFindOption, StatusRunning)
	}
	runs, jobs, err := GetConcurrentRunsAndJobs(ctx, actionRun.RepoID, actionRun.ConcurrencyGroup, statusFindOption)
	if err != nil {
		return nil, fmt.Errorf("find concurrent runs and jobs: %w", err)
	}
	jobsToCancel = append(jobsToCancel, jobs...)

	// cancel runs in the same concurrency group
	for _, run := range runs {
		if run.ID == actionRun.ID {
			continue
		}
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
