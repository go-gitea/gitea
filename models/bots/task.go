// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"errors"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"github.com/google/uuid"
	"xorm.io/builder"
)

func init() {
	db.RegisterModel(new(Task))
	db.RegisterModel(new(BuildIndex))
}

// TaskStatus represents a task status
type TaskStatus int

// enumerate all the statuses of bot task
const (
	TaskPending  TaskStatus = iota // wait for assign
	TaskAssigned                   // assigned to a runner
	TaskRunning                    // running
	TaskFailed
	TaskFinished
	TaskCanceled
	TaskTimeout
)

// Task represnets bot tasks
type Task struct {
	ID                int64
	Title             string
	UUID              string `xorm:"CHAR(36)"`
	Index             int64  `xorm:"index unique(repo_index)"`
	RepoID            int64  `xorm:"index unique(repo_index)"`
	TriggerUserID     int64
	TriggerUser       *user_model.User `xorm:"-"`
	Ref               string
	CommitSHA         string
	Event             webhook.HookEventType
	Token             string                           // token for this task
	Grant             string                           // permissions for this task
	EventPayload      string                           `xorm:"LONGTEXT"`
	RunnerID          int64                            `xorm:"index"`
	Status            TaskStatus                       `xorm:"index"`
	WorkflowsStatuses map[string]map[string]TaskStatus `xorm:"LONGTEXT"`
	Created           timeutil.TimeStamp               `xorm:"created"`
	StartTime         timeutil.TimeStamp
	EndTime           timeutil.TimeStamp
	Updated           timeutil.TimeStamp `xorm:"updated"`
}

func (t *Task) IsPending() bool {
	return t.Status == TaskPending || t.Status == TaskAssigned
}

func (t *Task) IsRunning() bool {
	return t.Status == TaskRunning
}

func (t *Task) IsFailed() bool {
	return t.Status == TaskFailed || t.Status == TaskCanceled || t.Status == TaskTimeout
}

func (t *Task) IsSuccess() bool {
	return t.Status == TaskFinished
}

// TableName represents a bot task
func (Task) TableName() string {
	return "bots_task"
}

func (t *Task) HTMLURL() string {
	return fmt.Sprintf("")
}

func updateRepoBuildsNumbers(ctx context.Context, repo *repo_model.Repository) error {
	_, err := db.GetEngine(ctx).ID(repo.ID).
		SetExpr("num_builds",
			builder.Select("count(*)").From("bots_task").
				Where(builder.Eq{"repo_id": repo.ID}),
		).
		SetExpr("num_closed_builds",
			builder.Select("count(*)").From("bots_task").
				Where(builder.Eq{
					"repo_id": repo.ID,
				}.And(
					builder.In("status", TaskFailed, TaskCanceled, TaskTimeout, TaskFinished),
				),
				),
		).
		Update(repo)
	return err
}

// InsertTask inserts a bot task
func InsertTask(t *Task) error {
	if t.UUID == "" {
		t.UUID = uuid.New().String()
	}
	index, err := db.GetNextResourceIndex("build_index", t.RepoID)
	if err != nil {
		return err
	}
	t.Index = index

	ctx, commiter, err := db.TxContext()
	if err != nil {
		return err
	}
	defer commiter.Close()

	if err := db.Insert(ctx, t); err != nil {
		return err
	}

	if err := updateRepoBuildsNumbers(ctx, &repo_model.Repository{ID: t.RepoID}); err != nil {
		return err
	}

	return commiter.Commit()
}

// UpdateTask updates bot task
func UpdateTask(t *Task, cols ...string) error {
	_, err := db.GetEngine(db.DefaultContext).ID(t.ID).Cols(cols...).Update(t)
	return err
}

// ErrTaskNotExist represents an error for bot task not exist
type ErrTaskNotExist struct {
	RepoID int64
	Index  int64
	UUID   string
}

func (err ErrTaskNotExist) Error() string {
	return fmt.Sprintf("Bot task [%s] is not exist", err.UUID)
}

// GetTaskByUUID gets bot task by uuid
func GetTaskByUUID(taskUUID string) (*Task, error) {
	var task Task
	has, err := db.GetEngine(db.DefaultContext).Where("uuid=?", taskUUID).Get(&task)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTaskNotExist{
			UUID: taskUUID,
		}
	}
	return &task, nil
}

// GetCurTaskByID return the task for the bot
func GetCurTaskByID(runnerID int64) (*Task, error) {
	var tasks []Task
	// FIXME: for test, just return all tasks
	err := db.GetEngine(db.DefaultContext).Where("status=?", TaskPending).Find(&tasks)
	// err := x.Where("runner_id = ?", botID).
	// And("status=?", BotTaskPending).
	// Find(&tasks)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, nil
	}
	return &tasks[0], err
}

// GetCurTaskByUUID return the task for the bot
func GetCurTaskByUUID(runnerUUID string) (*Task, error) {
	runner, err := GetRunnerByUUID(runnerUUID)
	if err != nil {
		return nil, err
	}
	return GetCurTaskByID(runner.ID)
}

func GetTaskByRepoAndIndex(repoID, index int64) (*Task, error) {
	var task Task
	has, err := db.GetEngine(db.DefaultContext).Where("repo_id=?", repoID).
		And("`index` = ?", index).
		Get(&task)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTaskNotExist{
			RepoID: repoID,
			Index:  index,
		}
	}
	return &task, nil
}

// AssignTaskToRunner assign a task to a runner
func AssignTaskToRunner(taskID int64, runnerID int64) error {
	cnt, err := db.GetEngine(db.DefaultContext).
		Where("runner_id=0").
		And("id=?", taskID).
		Cols("runner_id").
		Update(&Task{
			RunnerID: runnerID,
		})
	if err != nil {
		return err
	}
	if cnt != 1 {
		return errors.New("assign faild")
	}
	return nil
}

type FindTaskOptions struct {
	db.ListOptions
	RepoID   int64
	IsClosed util.OptionalBool
}

func (opts FindTaskOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.IsClosed.IsTrue() {
		cond = cond.And(builder.Expr("status IN (?,?,?,?)", TaskCanceled, TaskFailed, TaskTimeout, TaskFinished))
	} else if opts.IsClosed.IsFalse() {
		cond = cond.And(builder.Expr("status IN (?,?,?)", TaskPending, TaskAssigned, TaskRunning))
	}
	return cond
}

func FindTasks(opts FindTaskOptions) (TaskList, error) {
	sess := db.GetEngine(db.DefaultContext).Where(opts.toConds())
	if opts.ListOptions.PageSize > 0 {
		skip, take := opts.GetSkipTake()
		sess.Limit(take, skip)
	}
	var tasks []*Task
	return tasks, sess.Find(&tasks)
}

func CountTasks(opts FindTaskOptions) (int64, error) {
	return db.GetEngine(db.DefaultContext).Table("bots_task").Where(opts.toConds()).Count()
}

type TaskStage struct{}

type StageStep struct{}

type BuildIndex db.ResourceIndex
