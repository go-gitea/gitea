// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// ErrRunnerNotExist represents an error for bot runner not exist
type ErrRunnerNotExist struct {
	UUID string
}

func (err ErrRunnerNotExist) Error() string {
	return fmt.Sprintf("Bot runner [%s] is not exist", err.UUID)
}

// Runner represents runner machines
type Runner struct {
	ID          int64
	UUID        string `xorm:"CHAR(36) UNIQUE"`
	Name        string `xorm:"VARCHAR(32) UNIQUE"`
	OS          string `xorm:"VARCHAR(16) index"` // the runner running os
	Arch        string `xorm:"VARCHAR(16) index"` // the runner running architecture
	Type        string `xorm:"VARCHAR(16)"`
	OwnerID     int64  `xorm:"index"` // org level runner, 0 means system
	RepoID      int64  `xorm:"index"` // repo level runner, if orgid also is zero, then it's a global
	Description string `xorm:"TEXT"`
	Base        int    // 0 native 1 docker 2 virtual machine
	RepoRange   string // glob match which repositories could use this runner
	Token       string
	LastOnline  timeutil.TimeStamp `xorm:"index"`
	Created     timeutil.TimeStamp `xorm:"created"`
}

func (Runner) TableName() string {
	return "bots_runner"
}

func init() {
	db.RegisterModel(&Runner{})
}

type GetRunnerOptions struct {
	RepoID  int64
	OwnerID int64
}

func (opts GetRunnerOptions) toCond() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.OwnerID > 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	}
	cond = cond.Or(builder.Eq{"repo_id": 0, "owner_id": 0})
	return cond
}

// GetUsableRunner returns the usable runner
func GetUsableRunner(opts GetRunnerOptions) (*Runner, error) {
	var runner Runner
	has, err := db.GetEngine(db.DefaultContext).
		Where(opts.toCond()).
		Asc("last_online").
		Get(&runner)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrRunnerNotExist{}
	}

	return &runner, nil
}

// GetRunnerByUUID returns a bot runner via uuid
func GetRunnerByUUID(uuid string) (*Runner, error) {
	var runner Runner
	has, err := db.GetEngine(db.DefaultContext).Where("uuid=?", uuid).Get(&runner)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRunnerNotExist{
			UUID: uuid,
		}
	}
	return &runner, nil
}

// FindRunnersByRepoID returns all workers for the repository
func FindRunnersByRepoID(repoID int64) ([]*Runner, error) {
	var runners []*Runner
	err := db.GetEngine(db.DefaultContext).Where("repo_id=? OR repo_id=0", repoID).
		Find(&runners)
	if err != nil {
		return nil, err
	}
	err = db.GetEngine(db.DefaultContext).Join("INNER", "repository", "repository.owner_id = bot_runner.owner_id").Find(&runners)
	return runners, err
}
