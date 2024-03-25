// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

type ErrExternalAuthTokenNotExist struct {
	SessionID   string
	AuthTokenID string
}

func IsErrExternalAuthTokenNotExist(err error) bool {
	_, ok := err.(ErrExternalAuthTokenNotExist)
	return ok
}

func (err ErrExternalAuthTokenNotExist) Error() string {
	return fmt.Sprintf("external auth token does not exist [sessionID: %s, authTokenID: %s]", err.SessionID, err.AuthTokenID)
}

func (err ErrExternalAuthTokenNotExist) Unwrap() error {
	return util.ErrNotExist
}

type ExternalAuthToken struct {
	SessionID         string         `xorm:"pk"`
	AuthTokenID       string         `xorm:"INDEX"`
	UserID            int64          `xorm:"INDEX NOT NULL"`
	ExternalID        string         `xorm:"NOT NULL"`
	LoginSourceID     int64          `xorm:"INDEX NOT NULL"`
	RawData           map[string]any `xorm:"TEXT JSON"`
	AccessToken       string         `xorm:"TEXT"`
	AccessTokenSecret string         `xorm:"TEXT"`
	RefreshToken      string         `xorm:"TEXT"`
	ExpiresAt         time.Time
	IDToken           string `xorm:"TEXT"`
}

func init() {
	db.RegisterModel(new(ExternalAuthToken))
}

func InsertExternalAuthToken(ctx context.Context, t *ExternalAuthToken) error {
	_, err := db.GetEngine(ctx).Insert(t)
	return err
}

func GetExternalAuthTokenBySessionID(ctx context.Context, sessionID string) (*ExternalAuthToken, error) {
	t := &ExternalAuthToken{}
	has, err := db.GetEngine(ctx).ID(sessionID).Get(t)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrExternalAuthTokenNotExist{SessionID: sessionID}
	}
	return t, nil
}

func GetExternalAuthTokenByAuthTokenID(ctx context.Context, authTokenID string) (*ExternalAuthToken, error) {
	t := &ExternalAuthToken{}
	has, err := db.GetEngine(ctx).Where(builder.Eq{"auth_token_id": authTokenID}).Get(t)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrExternalAuthTokenNotExist{AuthTokenID: authTokenID}
	}
	return t, nil
}

func GetExternalAuthTokenSessionIDsAndAuthTokenIDs(ctx context.Context, userID, loginSourceID int64) ([]*ExternalAuthToken, error) {
	tlist := []*ExternalAuthToken{}
	cond := builder.NewCond().And(builder.Eq{"user_id": userID})
	if loginSourceID > 0 {
		cond = cond.And(builder.Eq{"login_source_id": loginSourceID})
	}
	if err := db.GetEngine(ctx).Cols("session_id", "auth_token_id").Where(cond).Find(&tlist); err != nil {
		return nil, err
	}
	return tlist, nil
}

func UpdateExternalAuthTokenBySessionID(ctx context.Context, sessionID string, t *ExternalAuthToken) error {
	_, err := db.GetEngine(ctx).ID(sessionID).AllCols().Update(t)
	return err
}

func DeleteExternalAuthTokenBySessionID(ctx context.Context, sessionID string) error {
	_, err := db.GetEngine(ctx).ID(sessionID).Delete(&ExternalAuthToken{})
	return err
}

func DeleteExternalAuthTokensByUserLoginSourceID(ctx context.Context, userID, loginSourceID int64) error {
	_, err := db.GetEngine(ctx).Where(builder.Eq{"user_id": userID, "login_source_id": loginSourceID}).Delete(&ExternalAuthToken{})
	return err
}

func DeleteExternalAuthTokensByUserID(ctx context.Context, userID int64) error {
	_, err := db.GetEngine(ctx).Where(builder.Eq{"user_id": userID}).Delete(&ExternalAuthToken{})
	return err
}

type FindExternalAuthTokenOptions struct {
	db.ListOptions
	UserID        int64
	ExternalID    string
	LoginSourceID int64
	OrderBy       string
}

func (opts FindExternalAuthTokenOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.UserID > 0 {
		cond = cond.And(builder.Eq{"user_id": opts.UserID})
	}
	if len(opts.ExternalID) > 0 {
		cond = cond.And(builder.Eq{"external_id": opts.ExternalID})
	}
	if opts.LoginSourceID > 0 {
		cond = cond.And(builder.Eq{"login_source_id": opts.LoginSourceID})
	}
	return cond
}

func (opts FindExternalAuthTokenOptions) ToOrders() string {
	return opts.OrderBy
}
