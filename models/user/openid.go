// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"errors"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"
)

// ErrOpenIDNotExist openid is not known
var ErrOpenIDNotExist = errors.New("OpenID is unknown")

// UserOpenID is the list of all OpenID identities of a user.
// Since this is a middle table, name it OpenID is not suitable, so we ignore the lint here
type UserOpenID struct { //revive:disable-line:exported
	ID   int64  `xorm:"pk autoincr"`
	UID  int64  `xorm:"INDEX NOT NULL"`
	URI  string `xorm:"UNIQUE NOT NULL"`
	Show bool   `xorm:"DEFAULT false"`
}

func init() {
	db.RegisterModel(new(UserOpenID))
}

// GetUserOpenIDs returns all openid addresses that belongs to given user.
func GetUserOpenIDs(uid int64) ([]*UserOpenID, error) {
	openids := make([]*UserOpenID, 0, 5)
	if err := db.GetEngine(db.DefaultContext).
		Where("uid=?", uid).
		Asc("id").
		Find(&openids); err != nil {
		return nil, err
	}

	return openids, nil
}

// isOpenIDUsed returns true if the openid has been used.
func isOpenIDUsed(ctx context.Context, uri string) (bool, error) {
	if len(uri) == 0 {
		return true, nil
	}

	return db.GetEngine(ctx).Get(&UserOpenID{URI: uri})
}

// ErrOpenIDAlreadyUsed represents a "OpenIDAlreadyUsed" kind of error.
type ErrOpenIDAlreadyUsed struct {
	OpenID string
}

// IsErrOpenIDAlreadyUsed checks if an error is a ErrOpenIDAlreadyUsed.
func IsErrOpenIDAlreadyUsed(err error) bool {
	_, ok := err.(ErrOpenIDAlreadyUsed)
	return ok
}

func (err ErrOpenIDAlreadyUsed) Error() string {
	return fmt.Sprintf("OpenID already in use [oid: %s]", err.OpenID)
}

func (err ErrOpenIDAlreadyUsed) Unwrap() error {
	return util.ErrAlreadyExist
}

// AddUserOpenID adds an pre-verified/normalized OpenID URI to given user.
// NOTE: make sure openid.URI is normalized already
func AddUserOpenID(ctx context.Context, openid *UserOpenID) error {
	used, err := isOpenIDUsed(ctx, openid.URI)
	if err != nil {
		return err
	} else if used {
		return ErrOpenIDAlreadyUsed{openid.URI}
	}

	return db.Insert(ctx, openid)
}

// DeleteUserOpenID deletes an openid address of given user.
func DeleteUserOpenID(openid *UserOpenID) (err error) {
	var deleted int64
	// ask to check UID
	address := UserOpenID{
		UID: openid.UID,
	}
	if openid.ID > 0 {
		deleted, err = db.GetEngine(db.DefaultContext).ID(openid.ID).Delete(&address)
	} else {
		deleted, err = db.GetEngine(db.DefaultContext).
			Where("openid=?", openid.URI).
			Delete(&address)
	}

	if err != nil {
		return err
	} else if deleted != 1 {
		return ErrOpenIDNotExist
	}
	return nil
}

// ToggleUserOpenIDVisibility toggles visibility of an openid address of given user.
func ToggleUserOpenIDVisibility(id int64) (err error) {
	_, err = db.GetEngine(db.DefaultContext).Exec("update `user_open_id` set `show` = not `show` where `id` = ?", id)
	return err
}
