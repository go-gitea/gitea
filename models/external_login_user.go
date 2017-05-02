// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import "github.com/markbates/goth"

// ExternalLoginUser makes the connecting between some existing user and additional external login sources
type ExternalLoginUser struct {
	ExternalID    string `xorm:"pk NOT NULL"`
	UserID        int64  `xorm:"INDEX NOT NULL"`
	LoginSourceID int64  `xorm:"pk NOT NULL"`
}

// GetExternalLogin checks if a externalID in loginSourceID scope already exists
func GetExternalLogin(externalLoginUser *ExternalLoginUser) (bool, error) {
	return x.Get(externalLoginUser)
}

// ListAccountLinks returns a map with the ExternalLoginUser and its LoginSource
func ListAccountLinks(user *User) ([]*ExternalLoginUser, error) {
	externalAccounts := make([]*ExternalLoginUser, 0, 5)
	err := x.Where("user_id=?", user.ID).
		Desc("login_source_id").
		Find(&externalAccounts)

	if err != nil {
		return nil, err
	}

	return externalAccounts, nil
}

// LinkAccountToUser link the gothUser to the user
func LinkAccountToUser(user *User, gothUser goth.User) error {
	loginSource, err := GetActiveOAuth2LoginSourceByName(gothUser.Provider)
	if err != nil {
		return err
	}

	externalLoginUser := &ExternalLoginUser{
		ExternalID:    gothUser.UserID,
		UserID:        user.ID,
		LoginSourceID: loginSource.ID,
	}
	has, err := x.Get(externalLoginUser)
	if err != nil {
		return err
	} else if has {
		return ErrExternalLoginUserAlreadyExist{gothUser.UserID, user.ID, loginSource.ID}
	}

	_, err = x.Insert(externalLoginUser)
	return err
}

// RemoveAccountLink will remove all external login sources for the given user
func RemoveAccountLink(user *User, loginSourceID int64) (int64, error) {
	deleted, err := x.Delete(&ExternalLoginUser{UserID: user.ID, LoginSourceID: loginSourceID})
	if err != nil {
		return deleted, err
	}
	if deleted < 1 {
		return deleted, ErrExternalLoginUserNotExist{user.ID, loginSourceID}
	}
	return deleted, err
}

// removeAllAccountLinks will remove all external login sources for the given user
func removeAllAccountLinks(e Engine, user *User) error {
	_, err := e.Delete(&ExternalLoginUser{UserID: user.ID})
	return err
}
