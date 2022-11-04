// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"
	"xorm.io/builder"

	gouuid "github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru"
	"github.com/nektos/act/pkg/jobparser"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Task represents a distribution of job
type Task struct {
	ID       int64
	JobID    int64
	Job      *RunJob     `xorm:"-"`
	Steps    []*TaskStep `xorm:"-"`
	Attempt  int64
	RunnerID int64 `xorm:"index"`
	Result   runnerv1.Result
	Status   Status             `xorm:"index"`
	Started  timeutil.TimeStamp `xorm:"index"`
	Stopped  timeutil.TimeStamp

	Token          string `xorm:"-"`
	TokenHash      string `xorm:"UNIQUE"` // sha256 of token
	TokenSalt      string
	TokenLastEight string `xorm:"token_last_eight"`

	LogFilename  string      // file name of log
	LogInStorage bool        // read log from database or from storage
	LogLength    int64       // lines count
	LogSize      int64       // blob size
	LogIndexes   *LogIndexes `xorm:"BLOB"` // line number to offset
	LogExpired   bool        // files that are too old will be deleted

	Created timeutil.TimeStamp `xorm:"created"`
	Updated timeutil.TimeStamp `xorm:"updated index"`
}

var successfulTokenTaskCache *lru.Cache

func init() {
	db.RegisterModel(new(Task), func() error {
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

func (Task) TableName() string {
	return "bots_task"
}

func (task *Task) TakeTime() time.Duration {
	if task.Started == 0 {
		return 0
	}
	started := task.Started.AsTime()
	if task.Status.IsDone() {
		return task.Stopped.AsTime().Sub(started)
	}
	task.Stopped.AsTime().Sub(started)
	return time.Since(started).Truncate(time.Second)
}

func (task *Task) LoadJob(ctx context.Context) error {
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
func (task *Task) LoadAttributes(ctx context.Context) error {
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

// FullSteps returns steps with "Set up job" and "Complete job"
func (task *Task) FullSteps() []*TaskStep {
	// TODO: The logic here is too complex and tricky, may need to be rewritten

	var firstStep, lastStep *TaskStep
	if l := len(task.Steps); l > 0 {
		firstStep = task.Steps[0]
		lastStep = task.Steps[l-1]
	}
	var index int64

	headStep := &TaskStep{
		Name:      "Set up job",
		LogLength: task.LogLength,
		Started:   task.Started,
		Status:    StatusWaiting,
	}
	if task.LogLength > 0 {
		headStep.Status = StatusRunning
	}
	if firstStep != nil && firstStep.LogLength > 0 {
		headStep.LogLength = firstStep.LogIndex
		headStep.Stopped = firstStep.Started
		headStep.Status = StatusSuccess
	}
	index += headStep.LogLength

	for _, step := range task.Steps {
		index += step.LogLength
	}

	tailStep := &TaskStep{
		Name:    "Complete job",
		Stopped: task.Stopped,
		Status:  StatusWaiting,
	}
	if lastStep != nil && lastStep.Result != runnerv1.Result_RESULT_UNSPECIFIED {
		tailStep.LogIndex = index
		tailStep.LogLength = task.LogLength - tailStep.LogIndex
		tailStep.Started = lastStep.Stopped
		tailStep.Status = StatusSuccess
	}
	steps := make([]*TaskStep, 0, len(task.Steps)+2)
	steps = append(steps, headStep)
	steps = append(steps, task.Steps...)
	steps = append(steps, tailStep)

	return steps
}

func (task *Task) GenerateToken() error {
	salt, err := util.CryptoRandomString(10)
	if err != nil {
		return err
	}
	task.TokenSalt = salt
	task.Token = base.EncodeSha1(gouuid.New().String())
	task.TokenHash = auth_model.HashToken(task.Token, task.TokenSalt)
	task.TokenLastEight = task.Token[len(task.Token)-8:]
	return nil
}

type LogIndexes []int64

func (i *LogIndexes) FromDB(b []byte) error {
	reader := bytes.NewReader(b)
	for {
		v, err := binary.ReadVarint(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("binary ReadVarint: %w", err)
		}
		*i = append(*i, v)
	}
}

func (i *LogIndexes) ToDB() ([]byte, error) {
	var buf []byte
	for _, v := range *i {
		buf = binary.AppendVarint(buf, v)
	}
	return buf, nil
}

func GetTaskByID(ctx context.Context, id int64) (*Task, error) {
	var task Task
	has, err := db.GetEngine(ctx).Where("id=?", id).Get(&task)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("task with id %d: %w", id, util.ErrNotExist)
	}

	return &task, nil
}

func GetTaskByToken(ctx context.Context, token string) (*Task, error) {
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
		task := &Task{
			TokenLastEight: lastEight,
		}
		// Re-get the task from the db in case it has been deleted in the intervening period
		has, err := db.GetEngine(db.DefaultContext).ID(id).Get(task)
		if err != nil {
			return nil, err
		}
		if has {
			return task, nil
		}
		successfulTokenTaskCache.Remove(token)
	}

	var tasks []*Task
	err := db.GetEngine(ctx).Where("token_last_eight = ?", lastEight).Find(&tasks)
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

func CreateTaskForRunner(ctx context.Context, runner *Runner) (*Task, bool, error) {
	dbCtx, commiter, err := db.TxContext()
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
		jobCond = builder.In("run_id", builder.Select("id").From(Run{}.TableName()).Where(jobCond))
	}

	var jobs []*RunJob
	if err := e.Where("task_id=? AND status=?", 0, StatusWaiting).And(jobCond).Asc("id").Find(&jobs); err != nil {
		return nil, false, err
	}

	// TODO: a more efficient way to filter labels
	var job *RunJob
	labels := append(runner.AgentLabels, runner.CustomLabels...)
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

	task := &Task{
		JobID:    job.ID,
		Attempt:  job.Attempt,
		RunnerID: runner.ID,
		Started:  now,
		Status:   StatusRunning,
	}
	if err := task.GenerateToken(); err != nil {
		return nil, false, err
	}

	var workflowJob *jobparser.Job
	if gots, err := jobparser.Parse(job.WorkflowPayload); err != nil {
		return nil, false, fmt.Errorf("parse workflow of job %d: %w", job.ID, err)
	} else if len(gots) != 1 {
		return nil, false, fmt.Errorf("workflow of job %d: not signle workflow", job.ID)
	} else {
		_, workflowJob = gots[0].Job()
	}

	if _, err := e.Insert(task); err != nil {
		return nil, false, err
	}

	task.LogFilename = logFileName(job.Run.Repo.FullName(), task.ID)
	if _, err := e.ID(task.ID).Cols("log_filename").Update(task); err != nil {
		return nil, false, err
	}

	steps := make([]*TaskStep, len(workflowJob.Steps))
	for i, v := range workflowJob.Steps {
		steps[i] = &TaskStep{
			Name:   v.String(),
			TaskID: task.ID,
			Number: int64(i),
			Status: StatusWaiting,
		}
	}
	if _, err := e.Insert(steps); err != nil {
		return nil, false, err
	}
	task.Steps = steps

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

func UpdateTask(ctx context.Context, task *Task, cols ...string) error {
	sess := db.GetEngine(ctx).ID(task.ID)
	if len(cols) > 0 {
		sess.Cols(cols...)
	}
	_, err := sess.Update(task)
	return err
}

func UpdateTaskByState(state *runnerv1.TaskState) (*Task, error) {
	stepStates := map[int64]*runnerv1.StepState{}
	for _, v := range state.Steps {
		stepStates[v.Id] = v
	}

	ctx, commiter, err := db.TxContext()
	if err != nil {
		return nil, err
	}
	defer commiter.Close()

	e := db.GetEngine(ctx)

	task := &Task{}
	if _, err := e.ID(state.Id).Get(task); err != nil {
		return nil, err
	}

	task.Result = state.Result
	if task.Result != runnerv1.Result_RESULT_UNSPECIFIED {
		task.Status = Status(task.Result)
		task.Stopped = timeutil.TimeStamp(state.StoppedAt.AsTime().Unix())
		if _, err := UpdateRunJob(ctx, &RunJob{
			ID:      task.JobID,
			Status:  task.Status,
			Stopped: task.Stopped,
		}, nil); err != nil {
			return nil, err
		}
	}

	if _, err := e.ID(task.ID).Update(task); err != nil {
		return nil, err
	}

	if err := task.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	prevStepDone := true
	for _, step := range task.Steps {
		if v, ok := stepStates[step.Number]; ok {
			step.Result = v.Result
			step.LogIndex = v.LogIndex
			step.LogLength = v.LogLength
			step.Started = convertTimestamp(v.StartedAt)
			step.Stopped = convertTimestamp(v.StoppedAt)
		}
		if step.Result != runnerv1.Result_RESULT_UNSPECIFIED {
			step.Status = Status(step.Result)
			prevStepDone = true
		} else if prevStepDone {
			step.Status = StatusRunning
			prevStepDone = false
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

func StopTask(ctx context.Context, task *Task, result runnerv1.Result) (*Task, error) {
	ctx, commiter, err := db.TxContext()
	if err != nil {
		return nil, err
	}
	defer commiter.Close()

	e := db.GetEngine(ctx)

	now := timeutil.TimeStampNow()
	task.Result = result
	if task.Result != runnerv1.Result_RESULT_UNSPECIFIED {
		task.Status = Status(task.Result)
		task.Stopped = now
		if _, err := UpdateRunJob(ctx, &RunJob{
			ID:      task.JobID,
			Status:  task.Status,
			Stopped: task.Stopped,
		}, nil); err != nil {
			return nil, err
		}
	}

	if _, err := e.ID(task.ID).Update(task); err != nil {
		return nil, err
	}

	if err := task.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	for _, step := range task.Steps {
		if step.Result == runnerv1.Result_RESULT_UNSPECIFIED {
			step.Result = result
			step.Status = Status(result)
			if step.Started == 0 {
				step.Started = now
			}
			step.Stopped = now
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

func isSubset(set, subset []string) bool {
	m := make(map[string]struct{}, len(set))
	for _, v := range set {
		m[v] = struct{}{}
	}
	for _, v := range subset {
		if _, ok := m[v]; !ok {
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
