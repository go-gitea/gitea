// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"crypto/subtle"
	"fmt"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	lru "github.com/hashicorp/golang-lru"
	"github.com/nektos/act/pkg/jobparser"
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
	Stopped  timeutil.TimeStamp

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
	LogIndexes   LogIndexes `xorm:"LONGBLOB"` // line number to offset
	LogExpired   bool       // files that are too old will be deleted

	Created timeutil.TimeStamp `xorm:"created"`
	Updated timeutil.TimeStamp `xorm:"updated index"`
}

var successfulTokenTaskCache *lru.Cache

func init() {
	db.RegisterModel(new(ActionTask), func() error {
		if setting.SuccessfulTokensCacheSize > 0 {
			var err error
			successfulTokenTaskCache, err = lru.New(setting.SuccessfulTokensCacheSize)
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
	return calculateDuration(task.Started, task.Stopped, task.Status)
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
		job, err := GetRunJobByID(ctx, task.JobID)
		if err != nil {
			return err
		}
		task.Job = job
	}
	return nil
}

// LoadAttributes load Job Steps if not loaded
func (task *ActionTask) LoadAttributes(ctx context.Context) error {
	if task == nil {
		return nil
	}
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

func (task *ActionTask) GenerateToken() (err error) {
	task.Token, task.TokenSalt, task.TokenHash, task.TokenLastEight, err = generateSaltedToken()
	return err
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
	err := db.GetEngine(ctx).Where("token_last_eight = ? AND status = ?", lastEight, StatusRunning).Find(&tasks)
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

func CreateTaskForRunner(ctx context.Context, runner *ActionRunner) (*ActionTask, bool, error) {
	dbCtx, commiter, err := db.TxContext(ctx)
	if err != nil {
		return nil, false, err
	}
	defer commiter.Close()
	ctx = dbCtx.WithContext(ctx)

	e := db.GetEngine(ctx)

	jobCond := builder.NewCond()
	if runner.RepoID != 0 {
		jobCond = builder.Eq{"repo_id": runner.RepoID}
	} else if runner.OwnerID != 0 {
		jobCond = builder.In("repo_id", builder.Select("id").From("repository").Where(builder.Eq{"owner_id": runner.OwnerID}))
	}
	if jobCond.IsValid() {
		jobCond = builder.In("run_id", builder.Select("id").From("action_run").Where(jobCond))
	}

	var jobs []*ActionRunJob
	if err := e.Where("task_id=? AND status=?", 0, StatusWaiting).And(jobCond).Asc("id").Find(&jobs); err != nil {
		return nil, false, err
	}

	// TODO: a more efficient way to filter labels
	var job *ActionRunJob
	labels := runner.AgentLabels
	labels = append(labels, runner.CustomLabels...)
	log.Trace("runner labels: %v", labels)
	for _, v := range jobs {
		if isSubset(labels, v.RunsOn) {
			job = v
			break
		}
	}
	if job == nil {
		return nil, false, nil
	}
	if err := job.LoadAttributes(ctx); err != nil {
		return nil, false, err
	}

	now := timeutil.TimeStampNow()
	job.Attempt++
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
	if err := task.GenerateToken(); err != nil {
		return nil, false, err
	}

	var workflowJob *jobparser.Job
	if gots, err := jobparser.Parse(job.WorkflowPayload); err != nil {
		return nil, false, fmt.Errorf("parse workflow of job %d: %w", job.ID, err)
	} else if len(gots) != 1 {
		return nil, false, fmt.Errorf("workflow of job %d: not single workflow", job.ID)
	} else {
		_, workflowJob = gots[0].Job()
	}

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
			name, _ := util.SplitStringAtByteN(v.String(), 255)
			steps[i] = &ActionTaskStep{
				Name:   name,
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

	if job.Run.Status.IsWaiting() {
		job.Run.Status = StatusRunning
		job.Run.Started = now
		if err := UpdateRun(ctx, job.Run, "status", "started"); err != nil {
			return nil, false, err
		}
	}

	task.Job = job

	if err := commiter.Commit(); err != nil {
		return nil, false, err
	}

	return task, true, nil
}

func UpdateTask(ctx context.Context, task *ActionTask, cols ...string) error {
	sess := db.GetEngine(ctx).ID(task.ID)
	if len(cols) > 0 {
		sess.Cols(cols...)
	}
	_, err := sess.Update(task)
	return err
}

// UpdateTaskByState updates the task by the state.
// It will always update the task if the state is not final, even there is no change.
// So it will update ActionTask.Updated to avoid the task being judged as a zombie task.
func UpdateTaskByState(ctx context.Context, state *runnerv1.TaskState) (*ActionTask, error) {
	stepStates := map[int64]*runnerv1.StepState{}
	for _, v := range state.Steps {
		stepStates[v.Id] = v
	}

	ctx, commiter, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer commiter.Close()

	e := db.GetEngine(ctx)

	task := &ActionTask{}
	if has, err := e.ID(state.Id).Get(task); err != nil {
		return nil, err
	} else if !has {
		return nil, util.ErrNotExist
	}

	if task.Status.IsDone() {
		// the state is final, do nothing
		return task, nil
	}

	// state.Result is not unspecified means the task is finished
	if state.Result != runnerv1.Result_RESULT_UNSPECIFIED {
		task.Status = Status(state.Result)
		task.Stopped = timeutil.TimeStamp(state.StoppedAt.AsTime().Unix())
		if err := UpdateTask(ctx, task, "status", "stopped"); err != nil {
			return nil, err
		}
		if _, err := UpdateRunJob(ctx, &ActionRunJob{
			ID:      task.JobID,
			Status:  task.Status,
			Stopped: task.Stopped,
		}, nil); err != nil {
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
			step.Status = Status(result)
		} else if step.Started != 0 {
			step.Status = StatusRunning
		}
		if _, err := e.ID(step.ID).Update(step); err != nil {
			return nil, err
		}
	}

	if err := commiter.Commit(); err != nil {
		return nil, err
	}

	return task, nil
}

func StopTask(ctx context.Context, taskID int64, status Status) error {
	if !status.IsDone() {
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
	task.Status = status
	task.Stopped = now
	if _, err := UpdateRunJob(ctx, &ActionRunJob{
		ID:      task.JobID,
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

func isSubset(set, subset []string) bool {
	m := make(container.Set[string], len(set))
	for _, v := range set {
		m.Add(v)
	}

	for _, v := range subset {
		if !m.Contains(v) {
			return false
		}
	}
	return true
}

func convertTimestamp(timestamp *timestamppb.Timestamp) timeutil.TimeStamp {
	if timestamp.GetSeconds() == 0 && timestamp.GetNanos() == 0 {
		return timeutil.TimeStamp(0)
	}
	return timeutil.TimeStamp(timestamp.AsTime().Unix())
}

func logFileName(repoFullName string, taskID int64) string {
	return fmt.Sprintf("%s/%02x/%d.log", repoFullName, taskID%256, taskID)
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
