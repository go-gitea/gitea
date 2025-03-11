// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/nektos/act/pkg/jobparser"
	"xorm.io/builder"
)

// ActionRun represents a run of a workflow file
type ActionRun struct {
	ID                int64
	Title             string
	RepoID            int64                  `xorm:"index unique(repo_index)"`
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
	return git.RefURL(run.Repo.Link(), run.Ref)
}

// PrettyRef return #id for pull ref or ShortName for others
func (run *ActionRun) PrettyRef() string {
	refName := git.RefName(run.Ref)
	if refName.IsPull() {
		return "#" + strings.TrimSuffix(strings.TrimPrefix(run.Ref, git.PullPrefix), "/head")
	}
	return refName.ShortName()
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
	if run.Event == webhook_module.HookEventPullRequest || run.Event == webhook_module.HookEventPullRequestSync {
		var payload api.PullRequestPayload
		if err := json.Unmarshal([]byte(run.EventPayload), &payload); err != nil {
			return nil, err
		}
		return &payload, nil
	}
	return nil, fmt.Errorf("event %s is not a pull request event", run.Event)
}

func (run *ActionRun) IsSchedule() bool {
	return run.ScheduleID > 0
}

func updateRepoRunsNumbers(ctx context.Context, repo *repo_model.Repository) error {
	_, err := db.GetEngine(ctx).ID(repo.ID).
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

				// If the update affected 0 rows, it means the job has changed in the meantime, so we need to try again.
				if n == 0 {
					return cancelledJobs, fmt.Errorf("job has changed, try again")
				}

				cancelledJobs = append(cancelledJobs, job)
				// Continue with the next job.
				continue
			}

			// If the job has an associated task, try to stop the task, effectively cancelling the job.
			if err := StopTask(ctx, job.TaskID, StatusCancelled); err != nil {
				return cancelledJobs, err
			}
			cancelledJobs = append(cancelledJobs, job)
		}
	}

	// Return nil to indicate successful cancellation of all running and waiting jobs.
	return cancelledJobs, nil
}

// InsertRun inserts a run
// The title will be cut off at 255 characters if it's longer than 255 characters.
func InsertRun(ctx context.Context, run *ActionRun, jobs []*jobparser.SingleWorkflow) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	index, err := db.GetNextResourceIndex(ctx, "action_run_index", run.RepoID)
	if err != nil {
		return err
	}
	run.Index = index
	run.Title, _ = util.SplitStringAtByteN(run.Title, 255)

	if err := db.Insert(ctx, run); err != nil {
		return err
	}

	if run.Repo == nil {
		repo, err := repo_model.GetRepositoryByID(ctx, run.RepoID)
		if err != nil {
			return err
		}
		run.Repo = repo
	}

	if err := updateRepoRunsNumbers(ctx, run.Repo); err != nil {
		return err
	}

	runJobs := make([]*ActionRunJob, 0, len(jobs))
	var hasWaiting bool
	for _, v := range jobs {
		id, job := v.Job()
		needs := job.Needs()
		if err := v.SetJob(id, job.EraseNeeds()); err != nil {
			return err
		}
		payload, _ := v.Marshal()
		status := StatusWaiting
		if len(needs) > 0 || run.NeedApproval {
			status = StatusBlocked
		} else {
			hasWaiting = true
		}
		job.Name, _ = util.SplitStringAtByteN(job.Name, 255)
		runJobs = append(runJobs, &ActionRunJob{
			RunID:             run.ID,
			RepoID:            run.RepoID,
			OwnerID:           run.OwnerID,
			CommitSHA:         run.CommitSHA,
			IsForkPullRequest: run.IsForkPullRequest,
			Name:              job.Name,
			WorkflowPayload:   payload,
			JobID:             id,
			Needs:             needs,
			RunsOn:            job.RunsOn(),
			Status:            status,
		})
	}
	if err := db.Insert(ctx, runJobs); err != nil {
		return err
	}

	// if there is a job in the waiting status, increase tasks version.
	if hasWaiting {
		if err := IncreaseTaskVersion(ctx, run.OwnerID, run.RepoID); err != nil {
			return err
		}
	}

	return committer.Commit()
}

func GetRunByID(ctx context.Context, id int64) (*ActionRun, error) {
	var run ActionRun
	has, err := db.GetEngine(ctx).Where("id=?", id).Get(&run)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("run with id %d: %w", id, util.ErrNotExist)
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
	run.Title, _ = util.SplitStringAtByteN(run.Title, 255)
	affected, err := sess.Update(run)
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("run has changed")
		// It's impossible that the run is not found, since Gitea never deletes runs.
	}

	if run.Status != 0 || slices.Contains(cols, "status") {
		if run.RepoID == 0 {
			run, err = GetRunByID(ctx, run.ID)
			if err != nil {
				return err
			}
		}
		if run.Repo == nil {
			repo, err := repo_model.GetRepositoryByID(ctx, run.RepoID)
			if err != nil {
				return err
			}
			run.Repo = repo
		}
		if err := updateRepoRunsNumbers(ctx, run.Repo); err != nil {
			return err
		}
	}

	return nil
}

type ActionRunIndex db.ResourceIndex
