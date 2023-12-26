// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

var ErrAuthTokenNotExist = util.NewNotExistErrorf("auth token does not exist")

type AuthToken struct { //nolint:revive
	ID          string `xorm:"pk"`
	TokenHash   string
	UserID      int64              `xorm:"INDEX"`
	ExpiresUnix timeutil.TimeStamp `xorm:"INDEX"`
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

func UpdateAuthTokenByID(ctx context.Context, t *AuthToken) error {
	_, err := db.GetEngine(ctx).ID(t.ID).Cols("token_hash", "expires_unix").Update(t)
	return err
}

func DeleteAuthTokenByID(ctx context.Context, id string) error {
	_, err := db.GetEngine(ctx).ID(id).Delete(&AuthToken{})
	return err
}

func DeleteExpiredAuthTokens(ctx context.Context) error {
	_, err := db.GetEngine(ctx).Where(builder.Lt{"expires_unix": timeutil.TimeStampNow()}).Delete(&AuthToken{})
	return err
}
