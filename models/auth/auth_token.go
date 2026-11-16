// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

var ErrAuthTokenNotExist = util.NewNotExistErrorf("auth token does not exist")

type AuthToken struct { //nolint:revive // export stutter
	ID          string `xorm:"pk"`
	TokenHash   string
	UserID      int64              `xorm:"INDEX"`
	ExpiresUnix timeutil.TimeStamp `xorm:"INDEX"`

	PreviousTokenHash string             // Identifies stale cookies without authenticating them.
	RotatedUnix       timeutil.TimeStamp // Bounds the grace window for stale cookies.
}

func init() {
	db.RegisterModel(new(AuthToken))
}

func InsertAuthToken(ctx context.Context, t *AuthToken) error {
	_, err := db.GetEngine(ctx).Insert(t)
	return err
}

func GetAuthTokenByID(ctx context.Context, id string) (*AuthToken, error) {
	at := &AuthToken{}

	has, err := db.GetEngine(ctx).ID(id).Get(at)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrAuthTokenNotExist
	}
	return at, nil
}

func UpdateAuthTokenByID(ctx context.Context, t *AuthToken, oldHash string) (bool, error) {
	affected, err := db.GetEngine(ctx).ID(t.ID).Where("token_hash = ?", oldHash).
		Cols("token_hash", "expires_unix", "previous_token_hash", "rotated_unix").Update(t)
	return affected == 1, err
}

func DeleteAuthTokenByID(ctx context.Context, id string) error {
	_, err := db.GetEngine(ctx).ID(id).Delete(&AuthToken{})
	return err
}

func DeleteAuthTokensByUserID(ctx context.Context, uid int64) error {
	_, err := db.GetEngine(ctx).Where(builder.Eq{"user_id": uid}).Delete(&AuthToken{})
	return err
}

func DeleteExpiredAuthTokens(ctx context.Context) error {
	_, err := db.GetEngine(ctx).Where(builder.Lt{"expires_unix": timeutil.TimeStampNow()}).Delete(&AuthToken{})
	return err
}
