// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"strings"

	"code.gitea.io/gitea/models/db"
)

// UserRedirect represents that a user name should be redirected to another
type UserRedirect struct {
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

// newUserRedirect create a new user redirect
func newUserRedirect(e db.Engine, ID int64, oldUserName, newUserName string) error {
	oldUserName = strings.ToLower(oldUserName)
	newUserName = strings.ToLower(newUserName)

	if err := deleteUserRedirect(e, newUserName); err != nil {
		return err
	}

	if _, err := e.Insert(&UserRedirect{
		LowerName:      oldUserName,
		RedirectUserID: ID,
	}); err != nil {
		return err
	}
	return nil
}

// deleteUserRedirect delete any redirect from the specified user name to
// anything else
func deleteUserRedirect(e db.Engine, userName string) error {
	userName = strings.ToLower(userName)
	_, err := e.Delete(&UserRedirect{LowerName: userName})
	return err
}
