// Copyright 2019 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/json"
	"fmt"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// Task represents a task
type Task struct {
	ID             int64
	DoerID         int64       `xorm:"index"` // operator
	Doer           *User       `xorm:"-"`
	OwnerID        int64       `xorm:"index"` // repo owner id, when creating, the repoID maybe zero
	Owner          *User       `xorm:"-"`
	RepoID         int64       `xorm:"index"`
	Repo           *Repository `xorm:"-"`
	Type           structs.TaskType
	Status         structs.TaskStatus `xorm:"index"`
	StartTime      timeutil.TimeStamp
	EndTime        timeutil.TimeStamp
	PayloadContent string             `xorm:"TEXT"`
	Errors         string             `xorm:"TEXT"` // if task failed, saved the error reason
	Created        timeutil.TimeStamp `xorm:"created"`
}

// LoadRepo loads repository of the task
func (task *Task) LoadRepo() error {
	return task.loadRepo(x)
}

func (task *Task) loadRepo(e Engine) error {
	if task.Repo != nil {
		return nil
	}
	var repo Repository
	has, err := e.ID(task.RepoID).Get(&repo)
	if err != nil {
		return err
	} else if !has {
		return ErrRepoNotExist{
			ID: task.RepoID,
		}
	}
	task.Repo = &repo
	return nil
}

// LoadDoer loads do user
func (task *Task) LoadDoer() error {
	if task.Doer != nil {
		return nil
	}

	var doer User
	has, err := x.ID(task.DoerID).Get(&doer)
	if err != nil {
		return err
	} else if !has {
		return ErrUserNotExist{
			UID: task.DoerID,
		}
	}
	task.Doer = &doer

	return nil
}

// LoadOwner loads owner user
func (task *Task) LoadOwner() error {
	if task.Owner != nil {
		return nil
	}

	var owner User
	has, err := x.ID(task.OwnerID).Get(&owner)
	if err != nil {
		return err
	} else if !has {
		return ErrUserNotExist{
			UID: task.OwnerID,
		}
	}
	task.Owner = &owner

	return nil
}

// UpdateCols updates some columns
func (task *Task) UpdateCols(cols ...string) error {
	_, err := x.ID(task.ID).Cols(cols...).Update(task)
	return err
}

// MigrateConfig returns task config when migrate repository
func (task *Task) MigrateConfig() (*structs.MigrateRepoOption, error) {
	if task.Type == structs.TaskTypeMigrateRepo {
		var opts structs.MigrateRepoOption
		err := json.Unmarshal([]byte(task.PayloadContent), &opts)
		if err != nil {
			return nil, err
		}
		return &opts, nil
	}
	return nil, fmt.Errorf("Task type is %s, not Migrate Repo", task.Type.Name())
}

// ErrTaskDoesNotExist represents a "TaskDoesNotExist" kind of error.
type ErrTaskDoesNotExist struct {
	ID     int64
	RepoID int64
	Type   structs.TaskType
}

// IsErrTaskDoesNotExist checks if an error is a ErrTaskIsNotExist.
func IsErrTaskDoesNotExist(err error) bool {
	_, ok := err.(ErrTaskDoesNotExist)
	return ok
}

func (err ErrTaskDoesNotExist) Error() string {
	return fmt.Sprintf("task is not exist [id: %d, repo_id: %d, type: %d]",
		err.ID, err.RepoID, err.Type)
}

// GetMigratingTask returns the migrating task by repo's id
func GetMigratingTask(repoID int64) (*Task, error) {
	var task = Task{
		RepoID: repoID,
		Type:   structs.TaskTypeMigrateRepo,
	}
	has, err := x.Get(&task)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTaskDoesNotExist{0, repoID, task.Type}
	}
	return &task, nil
}

// FindTaskOptions find all tasks
type FindTaskOptions struct {
	Status int
}

// ToConds generates conditions for database operation.
func (opts FindTaskOptions) ToConds() builder.Cond {
	var cond = builder.NewCond()
	if opts.Status >= 0 {
		cond = cond.And(builder.Eq{"status": opts.Status})
	}
	return cond
}

// FindTasks find all tasks
func FindTasks(opts FindTaskOptions) ([]*Task, error) {
	var tasks = make([]*Task, 0, 10)
	err := x.Where(opts.ToConds()).Find(&tasks)
	return tasks, err
}

func createTask(e Engine, task *Task) error {
	_, err := e.Insert(task)
	return err
}

// CreateMigrateTask creates a migrate task
func CreateMigrateTask(doer, u *User, opts base.MigrateOptions) (*Task, error) {
	bs, err := json.Marshal(&opts)
	if err != nil {
		return nil, err
	}

	var task = Task{
		DoerID:         doer.ID,
		OwnerID:        u.ID,
		Type:           structs.TaskTypeMigrateRepo,
		Status:         structs.TaskStatusQueue,
		PayloadContent: string(bs),
	}

	if err := createTask(x, &task); err != nil {
		return nil, err
	}

	repo, err := CreateRepository(doer, u, CreateRepoOptions{
		Name:        opts.RepoName,
		Description: opts.Description,
		OriginalURL: opts.OriginalURL,
		IsPrivate:   opts.Private,
		IsMirror:    opts.Mirror,
		Status:      RepositoryBeingMigrated,
	})
	if err != nil {
		task.EndTime = timeutil.TimeStampNow()
		task.Status = structs.TaskStatusFailed
		err2 := task.UpdateCols("end_time", "status")
		if err2 != nil {
			log.Error("UpdateCols Failed: %v", err2.Error())
		}
		return nil, err
	}

	task.RepoID = repo.ID
	if err = task.UpdateCols("repo_id"); err != nil {
		return nil, err
	}

	return &task, nil
}

// FinishMigrateTask updates database when migrate task finished
func FinishMigrateTask(task *Task) error {
	task.Status = structs.TaskStatusFinished
	task.EndTime = timeutil.TimeStampNow()
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if _, err := sess.ID(task.ID).Cols("status", "end_time").Update(task); err != nil {
		return err
	}
	task.Repo.Status = RepositoryReady
	if _, err := sess.ID(task.RepoID).Cols("status").Update(task.Repo); err != nil {
		return err
	}

	return sess.Commit()
}
