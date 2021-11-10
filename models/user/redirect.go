// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
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

// UserRedirect represents that a user name should be redirected to another
type UserRedirect struct { // nolint
	ID             int64  `xorm:"pk autoincr"`
	LowerName      string `xorm:"UNIQUE(s) INDEX NOT NULL"`
	RedirectUserID int64  // userID to redirect to
}

func init() {
	db.RegisterModel(new(UserRedirect))
}

// LookupUserRedirect look up userID if a user has a redirect name
func LookupUserRedirect(userName string) (int64, error) {
	userName = strings.ToLower(userName)
	redirect := &UserRedirect{LowerName: userName}
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

	if err := DeleteUserRedirect(ctx, newUserName); err != nil {
		return err
	}

	return db.Insert(ctx, &UserRedirect{
		LowerName:      oldUserName,
		RedirectUserID: ID,
	})
}

// DeleteUserRedirect delete any redirect from the specified user name to
// anything else
func DeleteUserRedirect(ctx context.Context, userName string) error {
	userName = strings.ToLower(userName)
	_, err := db.GetEngine(ctx).Delete(&UserRedirect{LowerName: userName})
	return err
}
