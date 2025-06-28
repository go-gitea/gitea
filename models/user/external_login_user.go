// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrExternalLoginUserAlreadyExist represents a "ExternalLoginUserAlreadyExist" kind of error.
type ErrExternalLoginUserAlreadyExist struct {
	ExternalID    string
	UserID        int64
	LoginSourceID int64
}

// IsErrExternalLoginUserAlreadyExist checks if an error is a ExternalLoginUserAlreadyExist.
func IsErrExternalLoginUserAlreadyExist(err error) bool {
	_, ok := err.(ErrExternalLoginUserAlreadyExist)
	return ok
}

func (err ErrExternalLoginUserAlreadyExist) Error() string {
	return fmt.Sprintf("external login user already exists [externalID: %s, userID: %d, loginSourceID: %d]", err.ExternalID, err.UserID, err.LoginSourceID)
}

func (err ErrExternalLoginUserAlreadyExist) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrExternalLoginUserNotExist represents a "ExternalLoginUserNotExist" kind of error.
type ErrExternalLoginUserNotExist struct {
	UserID        int64
	LoginSourceID int64
}

// IsErrExternalLoginUserNotExist checks if an error is a ExternalLoginUserNotExist.
func IsErrExternalLoginUserNotExist(err error) bool {
	_, ok := err.(ErrExternalLoginUserNotExist)
	return ok
}

func (err ErrExternalLoginUserNotExist) Error() string {
	return fmt.Sprintf("external login user link does not exists [userID: %d, loginSourceID: %d]", err.UserID, err.LoginSourceID)
}

func (err ErrExternalLoginUserNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ExternalLoginUser makes the connecting between some existing user and additional external login sources
type ExternalLoginUser struct {
	ExternalID        string         `xorm:"pk NOT NULL"`
	UserID            int64          `xorm:"INDEX NOT NULL"`
	LoginSourceID     int64          `xorm:"pk NOT NULL"`
	RawData           map[string]any `xorm:"TEXT JSON"`
	Provider          string         `xorm:"index VARCHAR(25)"`
	Email             string
	Name              string
	FirstName         string
	LastName          string
	NickName          string
	Description       string
	AvatarURL         string `xorm:"TEXT"`
	Location          string
	AccessToken       string `xorm:"TEXT"`
	AccessTokenSecret string `xorm:"TEXT"`
	RefreshToken      string `xorm:"TEXT"`
	ExpiresAt         time.Time
}

type ExternalUserMigrated interface {
	GetExternalName() string
	GetExternalID() int64
}

type ExternalUserRemappable interface {
	GetUserID() int64
	RemapExternalUser(externalName string, externalID, userID int64) error
	ExternalUserMigrated
}

func init() {
	db.RegisterModel(new(ExternalLoginUser))
}

// GetExternalLogin checks if a externalID in loginSourceID scope already exists
func GetExternalLogin(ctx context.Context, externalLoginUser *ExternalLoginUser) (bool, error) {
	return db.GetEngine(ctx).Get(externalLoginUser)
}

// LinkExternalToUser link the external user to the user
func LinkExternalToUser(ctx context.Context, user *User, externalLoginUser *ExternalLoginUser) error {
	has, err := db.Exist[ExternalLoginUser](ctx, builder.Eq{
		"external_id":     externalLoginUser.ExternalID,
		"login_source_id": externalLoginUser.LoginSourceID,
	})
	if err != nil {
		return err
	} else if has {
		return ErrExternalLoginUserAlreadyExist{externalLoginUser.ExternalID, user.ID, externalLoginUser.LoginSourceID}
	}

	_, err = db.GetEngine(ctx).Insert(externalLoginUser)
	return err
}

// RemoveAccountLink will remove all external login sources for the given user
func RemoveAccountLink(ctx context.Context, user *User, loginSourceID int64) (int64, error) {
	deleted, err := db.GetEngine(ctx).Delete(&ExternalLoginUser{UserID: user.ID, LoginSourceID: loginSourceID})
	if err != nil {
		return deleted, err
	}
	if deleted < 1 {
		return deleted, ErrExternalLoginUserNotExist{user.ID, loginSourceID}
	}
	return deleted, err
}

// RemoveAllAccountLinks will remove all external login sources for the given user
func RemoveAllAccountLinks(ctx context.Context, user *User) error {
	_, err := db.GetEngine(ctx).Delete(&ExternalLoginUser{UserID: user.ID})
	return err
}

// GetUserIDByExternalUserID get user id according to provider and userID
func GetUserIDByExternalUserID(ctx context.Context, provider, userID string) (int64, error) {
	var id int64
	_, err := db.GetEngine(ctx).Table("external_login_user").
		Select("user_id").
		Where("provider=?", provider).
		And("external_id=?", userID).
		Get(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// UpdateExternalUserByExternalID updates an external user's information
func UpdateExternalUserByExternalID(ctx context.Context, external *ExternalLoginUser) error {
	has, err := db.Exist[ExternalLoginUser](ctx, builder.Eq{
		"external_id":     external.ExternalID,
		"login_source_id": external.LoginSourceID,
	})
	if err != nil {
		return err
	} else if !has {
		return ErrExternalLoginUserNotExist{external.UserID, external.LoginSourceID}
	}

	_, err = db.GetEngine(ctx).Where("external_id=? AND login_source_id=?", external.ExternalID, external.LoginSourceID).AllCols().Update(external)
	return err
}

// EnsureLinkExternalToUser link the external user to the user
func EnsureLinkExternalToUser(ctx context.Context, external *ExternalLoginUser) error {
	has, err := db.Exist[ExternalLoginUser](ctx, builder.Eq{
		"external_id":     external.ExternalID,
		"login_source_id": external.LoginSourceID,
	})
	if err != nil {
		return err
	}

	if has {
		_, err = db.GetEngine(ctx).Where("external_id=? AND login_source_id=?", external.ExternalID, external.LoginSourceID).AllCols().Update(external)
		return err
	}

	_, err = db.GetEngine(ctx).Insert(external)
	return err
}

// FindExternalUserOptions represents an options to find external users
type FindExternalUserOptions struct {
	db.ListOptions
	Provider        string
	UserID          int64
	LoginSourceID   int64
	HasRefreshToken bool
	Expired         bool
	OrderBy         string
}

func (opts FindExternalUserOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if len(opts.Provider) > 0 {
		cond = cond.And(builder.Eq{"provider": opts.Provider})
	}
	if opts.UserID > 0 {
		cond = cond.And(builder.Eq{"user_id": opts.UserID})
	}
	if opts.Expired {
		cond = cond.And(builder.Lt{"expires_at": time.Now()})
	}
	if opts.HasRefreshToken {
		cond = cond.And(builder.Neq{"refresh_token": ""})
	}
	if opts.LoginSourceID != 0 {
		cond = cond.And(builder.Eq{"login_source_id": opts.LoginSourceID})
	}
	return cond
}

func (opts FindExternalUserOptions) ToOrders() string {
	return opts.OrderBy
}

func IterateExternalLogin(ctx context.Context, opts FindExternalUserOptions, f func(ctx context.Context, u *ExternalLoginUser) error) error {
	return db.Iterate(ctx, opts.ToConds(), f)
}
