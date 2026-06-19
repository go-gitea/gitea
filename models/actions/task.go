// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	runnerv1 "gitea.dev/actions-proto-go/runner/v1"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/unit"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	lru "github.com/hashicorp/golang-lru/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
	"xorm.io/builder"
)

// ActionTask represents a distribution of job
type ActionTask struct {
	ID       int64
	JobID    int64
	Job      *ActionRunJob     `xorm:"-"`
	Steps    []*ActionTaskStep `xorm:"-"`
	Attempt  int64
	RunnerID int64              `xorm:"index"`
	Status   Status             `xorm:"index"`
	Started  timeutil.TimeStamp `xorm:"index"`
	Stopped  timeutil.TimeStamp `xorm:"index(stopped_log_expired)"`

	RepoID            int64  `xorm:"index"`
	OwnerID           int64  `xorm:"index"`
	CommitSHA         string `xorm:"index"`
	IsForkPullRequest bool

	Token          string `xorm:"-"`
	TokenHash      string `xorm:"UNIQUE"` // sha256 of token
	TokenSalt      string
	TokenLastEight string `xorm:"index token_last_eight"`

	LogFilename  string     // file name of log
	LogInStorage bool       // read log from database or from storage
	LogLength    int64      // lines count
	LogSize      int64      // blob size
	LogIndexes   LogIndexes `xorm:"LONGBLOB"`                   // line number to offset
	LogExpired   bool       `xorm:"index(stopped_log_expired)"` // files that are too old will be deleted

	Created timeutil.TimeStamp `xorm:"created"`
	Updated timeutil.TimeStamp `xorm:"updated index"`
}

var successfulTokenTaskCache *lru.Cache[string, any]

func init() {
	db.RegisterModel(new(ActionTask), func() error {
		if setting.SuccessfulTokensCacheSize > 0 {
			var err error
			successfulTokenTaskCache, err = lru.New[string, any](setting.SuccessfulTokensCacheSize)
			if err != nil {
				return fmt.Errorf("unable to allocate Task cache: %v", err)
			}
		} else {
			successfulTokenTaskCache = nil
		}
		return nil
	})
}

func (task *ActionTask) Duration() time.Duration {
	return calculateDuration(task.Started, task.Stopped, task.Status, task.Updated)
}

func (task *ActionTask) IsStopped() bool {
	return task.Stopped > 0
}

func (task *ActionTask) GetRunLink() string {
	if task.Job == nil || task.Job.Run == nil {
		return ""
	}
	return task.Job.Run.Link()
}

func (task *ActionTask) GetCommitLink() string {
	if task.Job == nil || task.Job.Run == nil || task.Job.Run.Repo == nil {
		return ""
	}
	return task.Job.Run.Repo.CommitLink(task.CommitSHA)
}

func (task *ActionTask) GetRepoName() string {
	if task.Job == nil || task.Job.Run == nil || task.Job.Run.Repo == nil {
		return ""
	}
	return task.Job.Run.Repo.FullName()
}

func (task *ActionTask) GetRepoLink() string {
	if task.Job == nil || task.Job.Run == nil || task.Job.Run.Repo == nil {
		return ""
	}
	return task.Job.Run.Repo.Link()
}

func (task *ActionTask) LoadJob(ctx context.Context) error {
	if task.Job == nil {
		job, err := GetRunJobByRepoAndID(ctx, task.RepoID, task.JobID)
		if err != nil {
			return err
		}
		task.Job = job
	}
	return nil
}

// LoadAttributes load Job Steps if not loaded
func (task *ActionTask) LoadAttributes(ctx context.Context) error {
	if err := task.LoadJob(ctx); err != nil {
		return err
	}

	if err := task.Job.LoadAttributes(ctx); err != nil {
		return err
	}

	if task.Steps == nil { // be careful, an empty slice (not nil) also means loaded
		steps, err := GetTaskStepsByTaskID(ctx, task.ID)
		if err != nil {
			return err
		}
		task.Steps = steps
	}

	return nil
}

func (task *ActionTask) GenerateAndFillToken() {
	task.Token, task.TokenSalt, task.TokenHash, task.TokenLastEight = generateSaltedToken()
}

func GetTaskByID(ctx context.Context, id int64) (*ActionTask, error) {
	var task ActionTask
	has, err := db.GetEngine(ctx).Where("id=?", id).Get(&task)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("task with id %d: %w", id, util.ErrNotExist)
	}

	return &task, nil
}

func GetRunningTaskByToken(ctx context.Context, token string) (*ActionTask, error) {
	errNotExist := fmt.Errorf("task with token %q: %w", token, util.ErrNotExist)
	if token == "" {
		return nil, errNotExist
	}
	// A token is defined as being SHA1 sum these are 40 hexadecimal bytes long
	if len(token) != 40 {
		return nil, errNotExist
	}
	for _, x := range []byte(token) {
		if x < '0' || (x > '9' && x < 'a') || x > 'f' {
			return nil, errNotExist
		}
	}

	lastEight := token[len(token)-8:]

	if id := getTaskIDFromCache(token); id > 0 {
		task := &ActionTask{
			TokenLastEight: lastEight,
		}
		// Re-get the task from the db in case it has been deleted in the intervening period
		has, err := db.GetEngine(ctx).ID(id).Get(task)
		if err != nil {
			return nil, err
		}
		if has {
			return task, nil
		}
		successfulTokenTaskCache.Remove(token)
	}

	var tasks []*ActionTask
	// Cancelling tasks are still authenticating — post-run cleanup steps need API access (artifact uploads, cache saves, etc.) before the runner finalizes the task.
	err := db.GetEngine(ctx).Where("token_last_eight = ? AND status IN (?, ?)", lastEight, StatusRunning, StatusCancelling).Find(&tasks)
	if err != nil {
		return nil, err
	} else if len(tasks) == 0 {
		return nil, errNotExist
	}

	for _, t := range tasks {
		tempHash := auth_model.HashToken(token, t.TokenSalt)
		if subtle.ConstantTimeCompare([]byte(t.TokenHash), []byte(tempHash)) == 1 {
			if successfulTokenTaskCache != nil {
				successfulTokenTaskCache.Add(token, t.ID)
			}
			return t, nil
		}
	}
	return nil, errNotExist
}

func makeTaskStepDisplayName(step *jobparser.Step, limit int) (name string) {
	if step.Name != "" {
		name = step.Name // the step has an explicit name
	} else {
		// for unnamed step, its "String()" method tries to get a display name by its "name", "uses",
		// "run" or "id" (last fallback), we add the "Run " prefix for unnamed steps for better display
		// for multi-line "run" scripts, only use the first line to match GitHub's behavior
		// https://github.com/actions/runner/blob/66800900843747f37591b077091dd2c8cf2c1796/src/Runner.Worker/Handlers/ScriptHandler.cs#L45-L58
		runStr, _, _ := strings.Cut(strings.TrimSpace(step.Run), "\n")
		name = "Run " + util.IfZero(strings.TrimSpace(runStr), step.String())
	}
	return util.EllipsisDisplayString(name, limit) // database column has a length limit
}

func CreateTaskForRunner(ctx context.Context, runner *ActionRunner) (*ActionTask, bool, error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, false, err
	}
	defer committer.Close()

	e := db.GetEngine(ctx)

	jobCond := builder.NewCond()
	if runner.RepoID != 0 {
		jobCond = builder.Eq{"repo_id": runner.RepoID}
	} else if runner.OwnerID != 0 {
		jobCond = builder.In("repo_id", builder.Select("`repository`.id").From("repository").
			Join("INNER", "repo_unit", "`repository`.id = `repo_unit`.repo_id").
			Where(builder.Eq{"`repository`.owner_id": runner.OwnerID, "`repo_unit`.type": unit.TypeActions}))
	}
	if jobCond.IsValid() {
		jobCond = builder.In("run_id", builder.Select("id").From("action_run").Where(jobCond))
	}

	// Pick the oldest waiting job the runner's labels can run and prepare a task for
	// it. Label matching is pushed into SQL via the normalized action_run_job_label
	// table (runner labels must cover every required label), so the query stays
	// O(1 row) regardless of backlog size and never skips a matchable job behind an
	// unmatchable head.
	//
	// A job that can't be prepared — its run was deleted out from under it (#37586)
	// or its payload won't parse — is marked failed so it leaves the queue instead of
	// stalling every runner's poll, and the next candidate is tried. The attempt
	// bound keeps a single poll from clearing an unbounded backlog of such jobs.
	log.Trace("runner labels: %v", runner.AgentLabels)
	matchCond := runnerMatchableJobCond(runner.AgentLabels)
	for range maxTaskPickAttempts {
		job := new(ActionRunJob)
		has, err := e.Where(builder.Eq{"task_id": 0, "status": StatusWaiting, "is_reusable_caller": false}).
			And(jobCond).
			And(matchCond).
			Asc("updated", "id").
			Limit(1).
			Get(job)
		if err != nil {
			return nil, false, err
		}
		if !has {
			break
		}

		if err := job.LoadAttributes(ctx); err != nil {
			if !errors.Is(err, util.ErrNotExist) {
				return nil, false, err
			}
			// The run no longer exists (#37586); fail the orphaned job and move on.
			log.Warn("fail unpreparable action job %d (run %d): %v", job.ID, job.RunID, err)
			if err := failUnpreparableJob(ctx, job); err != nil {
				return nil, false, err
			}
			continue
		}

		workflowJob, err := job.ParseJob()
		if err != nil {
			// A job that never parses would otherwise stall the queue forever.
			log.Warn("fail unparsable action job %d: %v", job.ID, err)
			if err := failUnpreparableJob(ctx, job); err != nil {
				return nil, false, err
			}
			continue
		}

		task, claimed, err := assignJobToRunner(ctx, runner, job, workflowJob)
		if err != nil {
			return nil, false, err
		}
		if !claimed {
			// Another runner claimed this job between the select and the CAS. The task
			// and steps inserted by assignJobToRunner are discarded by the rollback; the
			// runner retries on its next poll.
			return nil, false, nil
		}

		if err := committer.Commit(); err != nil {
			return nil, false, err
		}
		return task, true, nil
	}

	// No assignable job (or we only found unpreparable ones); commit so any
	// fail-markings above persist, then report no task.
	if err := committer.Commit(); err != nil {
		return nil, false, err
	}
	return nil, false, nil
}

// maxTaskPickAttempts bounds how many candidate jobs a single assignment will
// skip past (failing unpreparable ones) before giving up for this poll.
const maxTaskPickAttempts = 10

// assignJobToRunner creates the task (and its steps) for job and claims the job
// for the runner via a task_id compare-and-swap. claimed is false when another
// runner won the job first; the caller must then roll back the transaction to
// discard the speculatively inserted task.
func assignJobToRunner(ctx context.Context, runner *ActionRunner, job *ActionRunJob, workflowJob *jobparser.Job) (*ActionTask, bool, error) {
	e := db.GetEngine(ctx)

	now := timeutil.TimeStampNow()
	job.Started = now
	job.Status = StatusRunning

	task := &ActionTask{
		JobID:             job.ID,
		Attempt:           job.Attempt,
		RunnerID:          runner.ID,
		Started:           now,
		Status:            StatusRunning,
		RepoID:            job.RepoID,
		OwnerID:           job.OwnerID,
		CommitSHA:         job.CommitSHA,
		IsForkPullRequest: job.IsForkPullRequest,
	}
	task.GenerateAndFillToken()

	if _, err := e.Insert(task); err != nil {
		return nil, false, err
	}

	task.LogFilename = logFileName(job.Run.Repo.FullName(), task.ID)
	if err := UpdateTask(ctx, task, "log_filename"); err != nil {
		return nil, false, err
	}

	if len(workflowJob.Steps) > 0 {
		steps := make([]*ActionTaskStep, len(workflowJob.Steps))
		for i, v := range workflowJob.Steps {
			steps[i] = &ActionTaskStep{
				Name:   makeTaskStepDisplayName(v, 255),
				TaskID: task.ID,
				Index:  int64(i),
				RepoID: task.RepoID,
				Status: StatusWaiting,
			}
		}
		if _, err := e.Insert(steps); err != nil {
			return nil, false, err
		}
		task.Steps = steps
	}

	job.TaskID = task.ID
	if n, err := UpdateRunJob(ctx, job, builder.Eq{"task_id": 0}); err != nil {
		return nil, false, err
	} else if n != 1 {
		return nil, false, nil
	}

	task.Job = job
	return task, true, nil
}

// failUnpreparableJob marks a waiting job failed with a direct row update,
// bypassing run/attempt aggregation: the run may have been deleted (#37586), in
// which case aggregation can't run, and the only goal is to get the job out of the
// waiting queue so it stops stalling task assignment. The CAS guards against a
// concurrent claim. If the run still exists its status self-heals when sibling
// jobs finish and aggregate this one.
func failUnpreparableJob(ctx context.Context, job *ActionRunJob) error {
	_, err := db.GetEngine(ctx).
		Where(builder.Eq{"id": job.ID, "task_id": 0, "status": StatusWaiting}).
		Cols("status", "stopped").
		Update(&ActionRunJob{Status: StatusFailure, Stopped: timeutil.TimeStampNow()})
	return err
}

func UpdateTask(ctx context.Context, task *ActionTask, cols ...string) error {
	sess := db.GetEngine(ctx).ID(task.ID)
	if len(cols) > 0 {
		sess.Cols(cols...)
	}
	_, err := sess.Update(task)

	// Automatically delete the ephemeral runner if the task is done
	if err == nil && task.Status.IsDone() && util.SliceContainsString(cols, "status") {
		return DeleteEphemeralRunner(ctx, task.RunnerID)
	}
	return err
}

// UpdateTaskByState updates the task by the state.
// It will always update the task if the state is not final, even there is no change.
// So it will update ActionTask.Updated to avoid the task being judged as a zombie task.
func UpdateTaskByState(ctx context.Context, runnerID int64, state *runnerv1.TaskState) (*ActionTask, error) {
	stepStates := map[int64]*runnerv1.StepState{}
	for _, v := range state.Steps {
		stepStates[v.Id] = v
	}

	return db.WithTx2(ctx, func(ctx context.Context) (*ActionTask, error) {
		e := db.GetEngine(ctx)

		task := &ActionTask{}
		if has, err := e.ID(state.Id).Get(task); err != nil {
			return nil, err
		} else if !has {
			return nil, util.ErrNotExist
		} else if runnerID != task.RunnerID {
			return nil, errors.New("invalid runner for task")
		}

		if task.Status.IsDone() {
			// the state is final, do nothing
			return task, nil
		}

		// state.Result is not unspecified means the task is finished
		if state.Result != runnerv1.Result_RESULT_UNSPECIFIED {
			if task.Status == StatusCancelling {
				// The runner may report SUCCESS/FAILURE for the cleanup phase; preserve user intent.
				task.Status = StatusCancelled
			} else {
				task.Status = StatusFromResult(state.Result)
			}
			task.Stopped = timeutil.TimeStamp(state.StoppedAt.AsTime().Unix())
			if err := UpdateTask(ctx, task, "status", "stopped"); err != nil {
				return nil, err
			}
			if _, err := UpdateRunJob(ctx, &ActionRunJob{
				ID:      task.JobID,
				RepoID:  task.RepoID,
				Status:  task.Status,
				Stopped: task.Stopped,
			}, nil, "status", "stopped"); err != nil {
				return nil, err
			}
		} else {
			// Force update ActionTask.Updated to avoid the task being judged as a zombie task
			task.Updated = timeutil.TimeStampNow()
			if err := UpdateTask(ctx, task, "updated"); err != nil {
				return nil, err
			}
		}

		if err := task.LoadAttributes(ctx); err != nil {
			return nil, err
		}

		for _, step := range task.Steps {
			var result runnerv1.Result
			if v, ok := stepStates[step.Index]; ok {
				result = v.Result
				step.LogIndex = v.LogIndex
				step.LogLength = v.LogLength
				step.Started = convertTimestamp(v.StartedAt)
				step.Stopped = convertTimestamp(v.StoppedAt)
			}
			if result != runnerv1.Result_RESULT_UNSPECIFIED {
				step.Status = StatusFromResult(result)
			} else if step.Started != 0 {
				step.Status = StatusRunning
			}
			if _, err := e.ID(step.ID).Update(step); err != nil {
				return nil, err
			}
		}

		return task, nil
	})
}

func StopTask(ctx context.Context, taskID int64, status Status) error {
	if !status.IsDone() && status != StatusCancelling {
		return fmt.Errorf("cannot stop task with status %v", status)
	}
	e := db.GetEngine(ctx)

	task := &ActionTask{}
	if has, err := e.ID(taskID).Get(task); err != nil {
		return err
	} else if !has {
		return util.ErrNotExist
	}
	if task.Status.IsDone() {
		return nil
	}

	now := timeutil.TimeStampNow()
	if status == StatusCancelling {
		runner, err := GetRunnerByID(ctx, task.RunnerID)
		if err != nil {
			if !errors.Is(err, util.ErrNotExist) {
				return err
			}
			status = StatusCancelled
		} else if !runner.HasCancellingSupport {
			status = StatusCancelled
		}
	}

	if status == StatusCancelling {
		task.Status = StatusCancelling

		if _, err := UpdateRunJob(ctx, &ActionRunJob{
			ID:     task.JobID,
			RepoID: task.RepoID,
			Status: StatusCancelling,
		}, nil, "status"); err != nil {
			return err
		}

		return UpdateTask(ctx, task, "status")
	}

	task.Status = status
	task.Stopped = now
	if _, err := UpdateRunJob(ctx, &ActionRunJob{
		ID:      task.JobID,
		RepoID:  task.RepoID,
		Status:  task.Status,
		Stopped: task.Stopped,
	}, nil); err != nil {
		return err
	}

	if err := UpdateTask(ctx, task, "status", "stopped"); err != nil {
		return err
	}

	if err := task.LoadAttributes(ctx); err != nil {
		return err
	}

	for _, step := range task.Steps {
		if !step.Status.IsDone() {
			step.Status = status
			if step.Started == 0 {
				step.Started = now
			}
			step.Stopped = now
		}
		if _, err := e.ID(step.ID).Update(step); err != nil {
			return err
		}
	}

	return nil
}

func FindOldTasksToExpire(ctx context.Context, olderThan timeutil.TimeStamp, limit int) ([]*ActionTask, error) {
	e := db.GetEngine(ctx)

	tasks := make([]*ActionTask, 0, limit)
	// Check "stopped > 0" to avoid deleting tasks that are still running
	return tasks, e.Where("stopped > 0 AND stopped < ? AND log_expired = ?", olderThan, false).
		Limit(limit).
		Find(&tasks)
}

func convertTimestamp(timestamp *timestamppb.Timestamp) timeutil.TimeStamp {
	if timestamp.GetSeconds() == 0 && timestamp.GetNanos() == 0 {
		return timeutil.TimeStamp(0)
	}
	return timeutil.TimeStamp(timestamp.AsTime().Unix())
}

func logFileName(repoFullName string, taskID int64) string {
	ret := fmt.Sprintf("%s/%02x/%d.log", repoFullName, taskID%256, taskID)

	if setting.Actions.LogCompression.IsZstd() {
		ret += ".zst"
	}

	return ret
}

func getTaskIDFromCache(token string) int64 {
	if successfulTokenTaskCache == nil {
		return 0
	}
	tInterface, ok := successfulTokenTaskCache.Get(token)
	if !ok {
		return 0
	}
	t, ok := tInterface.(int64)
	if !ok {
		return 0
	}
	return t
}
