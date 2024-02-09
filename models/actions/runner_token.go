// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// ActionRunnerToken represents runner tokens
type ActionRunnerToken struct {
	ID       int64
	Token    string                 `xorm:"UNIQUE"`
	OwnerID  int64                  `xorm:"index"` // org level runner, 0 means system
	Owner    *user_model.User       `xorm:"-"`
	RepoID   int64                  `xorm:"index"` // repo level runner, if orgid also is zero, then it's a global
	Repo     *repo_model.Repository `xorm:"-"`
	IsActive bool                   // true means it can be used

	Created timeutil.TimeStamp `xorm:"created"`
	Updated timeutil.TimeStamp `xorm:"updated"`
	Deleted timeutil.TimeStamp `xorm:"deleted"`
}

func init() {
	db.RegisterModel(new(ActionRunnerToken))
}

// GetRunnerToken returns a action runner via token
func GetRunnerToken(ctx context.Context, token string) (*ActionRunnerToken, error) {
	var runnerToken ActionRunnerToken
	has, err := db.GetEngine(ctx).Where("token=?", token).Get(&runnerToken)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("runner token %q: %w", token, util.ErrNotExist)
	}
	return &runnerToken, nil
}

// UpdateRunnerToken updates runner token information.
func UpdateRunnerToken(ctx context.Context, r *ActionRunnerToken, cols ...string) (err error) {
	e := db.GetEngine(ctx)

	if len(cols) == 0 {
		_, err = e.ID(r.ID).AllCols().Update(r)
	} else {
		_, err = e.ID(r.ID).Cols(cols...).Update(r)
	}
	return err
}

// NewRunnerToken creates a new active runner token and invalidate all old tokens
func NewRunnerToken(ctx context.Context, ownerID, repoID int64) (*ActionRunnerToken, error) {
	token, err := util.CryptoRandomString(40)
	if err != nil {
		return nil, err
	}
	runnerToken := &ActionRunnerToken{
		OwnerID:  ownerID,
		RepoID:   repoID,
		IsActive: true,
		Token:    token,
	}

	return runnerToken, db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Where("owner_id =? AND repo_id = ?", ownerID, repoID).Cols("is_active").Update(&ActionRunnerToken{
			IsActive: false,
		}); err != nil {
			return err
		}

		_, err = db.GetEngine(ctx).Insert(runnerToken)
		return err
	})
}

// GetLatestRunnerToken returns the latest runner token
func GetLatestRunnerToken(ctx context.Context, ownerID, repoID int64) (*ActionRunnerToken, error) {
	var runnerToken ActionRunnerToken
	has, err := db.GetEngine(ctx).Where("owner_id=? AND repo_id=?", ownerID, repoID).
		OrderBy("id DESC").Get(&runnerToken)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("runner token: %w", util.ErrNotExist)
	}
	return &runnerToken, nil
}
