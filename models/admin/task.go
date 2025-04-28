// Copyright 2019 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// Task represents a task
type Task struct {
	ID             int64
	DoerID         int64                  `xorm:"index"` // operator
	Doer           *user_model.User       `xorm:"-"`
	OwnerID        int64                  `xorm:"index"` // repo owner id, when creating, the repoID maybe zero
	Owner          *user_model.User       `xorm:"-"`
	RepoID         int64                  `xorm:"index"`
	Repo           *repo_model.Repository `xorm:"-"`
	Type           structs.TaskType
	Status         structs.TaskStatus `xorm:"index"`
	StartTime      timeutil.TimeStamp
	EndTime        timeutil.TimeStamp
	PayloadContent string             `xorm:"TEXT"`
	Message        string             `xorm:"TEXT"` // if task failed, saved the error reason, it could be a JSON string of TranslatableMessage or a plain message
	Created        timeutil.TimeStamp `xorm:"created"`
}

func init() {
	db.RegisterModel(new(Task))
}

// TranslatableMessage represents JSON struct that can be translated with a Locale
type TranslatableMessage struct {
	Format string
	Args   []any `json:",omitempty"`
}

// LoadRepo loads repository of the task
func (task *Task) LoadRepo(ctx context.Context) error {
	if task.Repo != nil {
		return nil
	}
	var repo repo_model.Repository
	has, err := db.GetEngine(ctx).ID(task.RepoID).Get(&repo)
	if err != nil {
		return err
	} else if !has {
		return repo_model.ErrRepoNotExist{
			ID: task.RepoID,
		}
	}
	task.Repo = &repo
	return nil
}

// LoadDoer loads do user
func (task *Task) LoadDoer(ctx context.Context) error {
	if task.Doer != nil {
		return nil
	}

	var doer user_model.User
	has, err := db.GetEngine(ctx).ID(task.DoerID).Get(&doer)
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
func (task *Task) LoadOwner(ctx context.Context) error {
	if task.Owner != nil {
		return nil
	}

	var owner user_model.User
	has, err := db.GetEngine(ctx).ID(task.OwnerID).Get(&owner)
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
func (task *Task) UpdateCols(ctx context.Context, cols ...string) error {
	_, err := db.GetEngine(ctx).ID(task.ID).Cols(cols...).Update(task)
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

// IsErrTaskDoesNotExist checks if an error is a ErrTaskDoesNotExist.
func IsErrTaskDoesNotExist(err error) bool {
	_, ok := err.(ErrTaskDoesNotExist)
	return ok
}

func (err ErrTaskDoesNotExist) Error() string {
	return fmt.Sprintf("task does not exist [id: %d, repo_id: %d, type: %d]",
		err.ID, err.RepoID, err.Type)
}

func (err ErrTaskDoesNotExist) Unwrap() error {
	return util.ErrNotExist
}

// GetMigratingTask returns the migrating task by repo's id
func GetMigratingTask(ctx context.Context, repoID int64) (*Task, error) {
	task := Task{
		RepoID: repoID,
		Type:   structs.TaskTypeMigrateRepo,
	}
	has, err := db.GetEngine(ctx).Get(&task)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTaskDoesNotExist{0, repoID, task.Type}
	}
	return &task, nil
}

// CreateTask creates a task on database
func CreateTask(ctx context.Context, task *Task) error {
	return db.Insert(ctx, task)
}

// FinishMigrateTask updates database when migrate task finished
func FinishMigrateTask(ctx context.Context, task *Task) error {
	task.Status = structs.TaskStatusFinished
	task.EndTime = timeutil.TimeStampNow()

	// delete credentials when we're done, they're a liability.
	conf, err := task.MigrateConfig()
	if err != nil {
		return err
	}
	conf.AuthPassword = ""
	conf.AuthToken = ""
	conf.CloneAddr = util.SanitizeCredentialURLs(conf.CloneAddr)
	conf.AuthPasswordEncrypted = ""
	conf.AuthTokenEncrypted = ""
	conf.CloneAddrEncrypted = ""
	confBytes, err := json.Marshal(conf)
	if err != nil {
		return err
	}
	task.PayloadContent = string(confBytes)

	_, err = db.GetEngine(ctx).ID(task.ID).Cols("status", "end_time", "payload_content").Update(task)
	return err
}
