// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"
)

// ErrUserRedirectNotExist represents a "UserRedirectNotExist" kind of error.
type ErrUserRedirectNotExist struct {
	Name string
}

// IsErrUserRedirectNotExist check if an error is an ErrUserRedirectNotExist.
func IsErrUserRedirectNotExist(err error) bool {
	_, ok := err.(ErrUserRedirectNotExist)
	return ok
}

func (err ErrUserRedirectNotExist) Error() string {
	return fmt.Sprintf("user redirect does not exist [name: %s]", err.Name)
}

func (err ErrUserRedirectNotExist) Unwrap() error {
	return util.ErrNotExist
}

// Redirect represents that a user name should be redirected to another
type Redirect struct {
	ID             int64  `xorm:"pk autoincr"`
	LowerName      string `xorm:"UNIQUE(s) INDEX NOT NULL"`
	RedirectUserID int64  // userID to redirect to
}

// TableName provides the real table name
func (Redirect) TableName() string {
	return "user_redirect"
}

func init() {
	db.RegisterModel(new(Redirect))
}

// LookupUserRedirect look up userID if a user has a redirect name
func LookupUserRedirect(userName string) (int64, error) {
	userName = strings.ToLower(userName)
	redirect := &Redirect{LowerName: userName}
	if has, err := db.GetEngine(db.DefaultContext).Get(redirect); err != nil {
		return 0, err
	} else if !has {
		return 0, ErrUserRedirectNotExist{Name: userName}
	}
	return redirect.RedirectUserID, nil
}

// NewUserRedirect create a new user redirect
func NewUserRedirect(ctx context.Context, ID int64, oldUserName, newUserName string) error {
	oldUserName = strings.ToLower(oldUserName)
	newUserName = strings.ToLower(newUserName)

	if err := DeleteUserRedirect(ctx, oldUserName); err != nil {
		return err
	}

	if err := DeleteUserRedirect(ctx, newUserName); err != nil {
		return err
	}

	return db.Insert(ctx, &Redirect{
		LowerName:      oldUserName,
		RedirectUserID: ID,
	})
}

// DeleteUserRedirect delete any redirect from the specified user name to
// anything else
func DeleteUserRedirect(ctx context.Context, userName string) error {
	userName = strings.ToLower(userName)
	_, err := db.GetEngine(ctx).Delete(&Redirect{LowerName: userName})
	return err
}
