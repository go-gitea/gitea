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
//
// It can be:
//  1. global token, OwnerID is 0 and RepoID is 0
//  2. org/user level token, OwnerID is org/user ID and RepoID is 0
//  3. repo level token, OwnerID is 0 and RepoID is repo ID
//
// Please note that it's not acceptable to have both OwnerID and RepoID to be non-zero,
// or it will be complicated to find tokens belonging to a specific owner.
// For example, conditions like `OwnerID = 1` will also return token {OwnerID: 1, RepoID: 1},
// but it's a repo level token, not an org/user level token.
// To avoid this, make it clear with {OwnerID: 0, RepoID: 1} for repo level tokens.
type ActionRunnerToken struct {
	ID       int64
	Token    string                 `xorm:"UNIQUE"`
	OwnerID  int64                  `xorm:"index"`
	Owner    *user_model.User       `xorm:"-"`
	RepoID   int64                  `xorm:"index"`
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
		return nil, fmt.Errorf(`runner token "%s...": %w`, util.TruncateRunes(token, 3), util.ErrNotExist)
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

// NewRunnerTokenWithValue creates a new active runner token and invalidate all old tokens
// ownerID will be ignored and treated as 0 if repoID is non-zero.
func NewRunnerTokenWithValue(ctx context.Context, ownerID, repoID int64, token string) (*ActionRunnerToken, error) {
	if ownerID != 0 && repoID != 0 {
		// It's trying to create a runner token that belongs to a repository, but OwnerID has been set accidentally.
		// Remove OwnerID to avoid confusion; it's not worth returning an error here.
		ownerID = 0
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

		_, err := db.GetEngine(ctx).Insert(runnerToken)
		return err
	})
}

func NewRunnerToken(ctx context.Context, ownerID, repoID int64) (*ActionRunnerToken, error) {
	token, err := util.CryptoRandomString(40)
	if err != nil {
		return nil, err
	}
	return NewRunnerTokenWithValue(ctx, ownerID, repoID, token)
}

// GetLatestRunnerToken returns the latest runner token
func GetLatestRunnerToken(ctx context.Context, ownerID, repoID int64) (*ActionRunnerToken, error) {
	if ownerID != 0 && repoID != 0 {
		// It's trying to get a runner token that belongs to a repository, but OwnerID has been set accidentally.
		// Remove OwnerID to avoid confusion; it's not worth returning an error here.
		ownerID = 0
	}

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
