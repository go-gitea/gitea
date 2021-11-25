// Copyright 2019 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// Task represents a task
type Task struct {
	ID             int64
	DoerID         int64            `xorm:"index"` // operator
	Doer           *user_model.User `xorm:"-"`
	OwnerID        int64            `xorm:"index"` // repo owner id, when creating, the repoID maybe zero
	Owner          *user_model.User `xorm:"-"`
	RepoID         int64            `xorm:"index"`
	Repo           *Repository      `xorm:"-"`
	Type           structs.TaskType
	Status         structs.TaskStatus `xorm:"index"`
	StartTime      timeutil.TimeStamp
	EndTime        timeutil.TimeStamp
	PayloadContent string             `xorm:"TEXT"`
	Message        string             `xorm:"TEXT"` // if task failed, saved the error reason
	Created        timeutil.TimeStamp `xorm:"created"`
}

func init() {
	db.RegisterModel(new(Task))
}

// TranslatableMessage represents JSON struct that can be translated with a Locale
type TranslatableMessage struct {
	Format string
	Args   []interface{} `json:"omitempty"`
}

// LoadRepo loads repository of the task
func (task *Task) LoadRepo() error {
	return task.loadRepo(db.GetEngine(db.DefaultContext))
}

func (task *Task) loadRepo(e db.Engine) error {
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

	var doer user_model.User
	has, err := db.GetEngine(db.DefaultContext).ID(task.DoerID).Get(&doer)
	if err != nil {
		return err
	} else if !has {
		return user_model.ErrUserNotExist{
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

	var owner user_model.User
	has, err := db.GetEngine(db.DefaultContext).ID(task.OwnerID).Get(&owner)
	if err != nil {
		return err
	} else if !has {
		return user_model.ErrUserNotExist{
			UID: task.OwnerID,
		}
	}
	task.Owner = &owner

	return nil
}

// UpdateCols updates some columns
func (task *Task) UpdateCols(cols ...string) error {
	_, err := db.GetEngine(db.DefaultContext).ID(task.ID).Cols(cols...).Update(task)
	return err
}

// MigrateConfig returns task config when migrate repository
func (task *Task) MigrateConfig() (*migration.MigrateOptions, error) {
	if task.Type == structs.TaskTypeMigrateRepo {
		var opts migration.MigrateOptions
		err := json.Unmarshal([]byte(task.PayloadContent), &opts)
		if err != nil {
			return nil, err
		}

		// decrypt credentials
		if opts.CloneAddrEncrypted != "" {
			if opts.CloneAddr, err = secret.DecryptSecret(setting.SecretKey, opts.CloneAddrEncrypted); err != nil {
				return nil, err
			}
		}
		if opts.AuthPasswordEncrypted != "" {
			if opts.AuthPassword, err = secret.DecryptSecret(setting.SecretKey, opts.AuthPasswordEncrypted); err != nil {
				return nil, err
			}
		}
		if opts.AuthTokenEncrypted != "" {
			if opts.AuthToken, err = secret.DecryptSecret(setting.SecretKey, opts.AuthTokenEncrypted); err != nil {
				return nil, err
			}
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
	task := Task{
		RepoID: repoID,
		Type:   structs.TaskTypeMigrateRepo,
	}
	has, err := db.GetEngine(db.DefaultContext).Get(&task)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTaskDoesNotExist{0, repoID, task.Type}
	}
	return &task, nil
}

// GetMigratingTaskByID returns the migrating task by repo's id
func GetMigratingTaskByID(id, doerID int64) (*Task, *migration.MigrateOptions, error) {
	task := Task{
		ID:     id,
		DoerID: doerID,
		Type:   structs.TaskTypeMigrateRepo,
	}
	has, err := db.GetEngine(db.DefaultContext).Get(&task)
	if err != nil {
		return nil, nil, err
	} else if !has {
		return nil, nil, ErrTaskDoesNotExist{id, 0, task.Type}
	}

	var opts migration.MigrateOptions
	if err := json.Unmarshal([]byte(task.PayloadContent), &opts); err != nil {
		return nil, nil, err
	}
	return &task, &opts, nil
}

// FindTaskOptions find all tasks
type FindTaskOptions struct {
	Status int
}

// ToConds generates conditions for database operation.
func (opts FindTaskOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.Status >= 0 {
		cond = cond.And(builder.Eq{"status": opts.Status})
	}
	return cond
}

// FindTasks find all tasks
func FindTasks(opts FindTaskOptions) ([]*Task, error) {
	tasks := make([]*Task, 0, 10)
	err := db.GetEngine(db.DefaultContext).Where(opts.ToConds()).Find(&tasks)
	return tasks, err
}

// CreateTask creates a task on database
func CreateTask(task *Task) error {
	return createTask(db.GetEngine(db.DefaultContext), task)
}

func createTask(e db.Engine, task *Task) error {
	_, err := e.Insert(task)
	return err
}

// FinishMigrateTask updates database when migrate task finished
func FinishMigrateTask(task *Task) error {
	task.Status = structs.TaskStatusFinished
	task.EndTime = timeutil.TimeStampNow()

	// delete credentials when we're done, they're a liability.
	conf, err := task.MigrateConfig()
	if err != nil {
		return err
	}
	conf.AuthPassword = ""
	conf.AuthToken = ""
	conf.CloneAddr = util.NewStringURLSanitizer(conf.CloneAddr, true).Replace(conf.CloneAddr)
	conf.AuthPasswordEncrypted = ""
	conf.AuthTokenEncrypted = ""
	conf.CloneAddrEncrypted = ""
	confBytes, err := json.Marshal(conf)
	if err != nil {
		return err
	}
	task.PayloadContent = string(confBytes)

	_, err = db.GetEngine(db.DefaultContext).ID(task.ID).Cols("status", "end_time", "payload_content").Update(task)
	return err
}
