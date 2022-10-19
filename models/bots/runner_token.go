// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/timeutil"

	gouuid "github.com/google/uuid"
)

// ErrRunnerNotExist represents an error for bot runner not exist
type ErrRunnerTokenNotExist struct {
	Token string
}

func (err ErrRunnerTokenNotExist) Error() string {
	return fmt.Sprintf("runner token [%s] is not exist", err.Token)
}

// RunnerToken represents runner tokens
type RunnerToken struct {
	ID       int64
	Token    string                 `xorm:"CHAR(36) UNIQUE"`
	OwnerID  int64                  `xorm:"index"` // org level runner, 0 means system
	Owner    *user_model.User       `xorm:"-"`
	RepoID   int64                  `xorm:"index"` // repo level runner, if orgid also is zero, then it's a global
	Repo     *repo_model.Repository `xorm:"-"`
	IsActive bool

	Created timeutil.TimeStamp `xorm:"created"`
	Updated timeutil.TimeStamp `xorm:"updated"`
	Deleted timeutil.TimeStamp `xorm:"deleted"`
}

func (RunnerToken) TableName() string {
	return "bots_runner_token"
}

func init() {
	db.RegisterModel(new(RunnerToken))
}

// GetRunnerByToken returns a bot runner via token
func GetRunnerToken(token string) (*RunnerToken, error) {
	var runnerToken RunnerToken
	has, err := db.GetEngine(db.DefaultContext).Where("token=?", token).Get(&runnerToken)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRunnerTokenNotExist{
			Token: token,
		}
	}
	return &runnerToken, nil
}

// UpdateRunnerToken updates runner token information.
func UpdateRunnerToken(ctx context.Context, r *RunnerToken, cols ...string) (err error) {
	e := db.GetEngine(ctx)

	if len(cols) == 0 {
		_, err = e.ID(r.ID).AllCols().Update(r)
	} else {
		_, err = e.ID(r.ID).Cols(cols...).Update(r)
	}
	return err
}

// NewRunnerToken creates a new runner token
func NewRunnerToken(ownerID, repoID int64) (*RunnerToken, error) {
	runnerToken := &RunnerToken{
		OwnerID:  ownerID,
		RepoID:   repoID,
		IsActive: false,
		// FIXME: why token is 36 chars?
		Token: base.EncodeSha1(gouuid.New().String())[:36],
	}
	_, err := db.GetEngine(db.DefaultContext).Insert(runnerToken)
	return runnerToken, err
}

// GetUnactivatedRunnerToken returns a unactivated runner token
func GetUnactivatedRunnerToken(ownerID, repoID int64) (*RunnerToken, error) {
	var runnerToken RunnerToken
	has, err := db.GetEngine(db.DefaultContext).Where("owner_id=? AND repo_id=? AND is_active=?", ownerID, repoID, false).OrderBy("id DESC").Get(&runnerToken)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRunnerTokenNotExist{}
	}
	return &runnerToken, nil
}
