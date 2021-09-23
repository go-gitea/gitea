// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package login

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
)

//  ________      _____          __  .__
//  \_____  \    /  _  \  __ ___/  |_|  |__
//   /   |   \  /  /_\  \|  |  \   __\  |  \
//  /    |    \/    |    \  |  /|  | |   Y  \
//  \_______  /\____|__  /____/ |__| |___|  /
//          \/         \/                 \/

// ErrOAuthClientIDInvalid will be thrown if client id cannot be found
type ErrOAuthClientIDInvalid struct {
	ClientID string
}

// IsErrOauthClientIDInvalid checks if an error is a ErrReviewNotExist.
func IsErrOauthClientIDInvalid(err error) bool {
	_, ok := err.(ErrOAuthClientIDInvalid)
	return ok
}

// Error returns the error message
func (err ErrOAuthClientIDInvalid) Error() string {
	return fmt.Sprintf("Client ID invalid [Client ID: %s]", err.ClientID)
}

// ErrOAuthApplicationNotFound will be thrown if id cannot be found
type ErrOAuthApplicationNotFound struct {
	ID int64
}

// IsErrOAuthApplicationNotFound checks if an error is a ErrReviewNotExist.
func IsErrOAuthApplicationNotFound(err error) bool {
	_, ok := err.(ErrOAuthApplicationNotFound)
	return ok
}

// Error returns the error message
func (err ErrOAuthApplicationNotFound) Error() string {
	return fmt.Sprintf("OAuth application not found [ID: %d]", err.ID)
}

// GetActiveOAuth2ProviderLoginSources returns all actived LoginOAuth2 sources
func GetActiveOAuth2ProviderLoginSources() ([]*Source, error) {
	sources := make([]*Source, 0, 1)
	if err := db.GetEngine(db.DefaultContext).Where("is_active = ? and type = ?", true, OAuth2).Find(&sources); err != nil {
		return nil, err
	}
	return sources, nil
}

// GetActiveOAuth2LoginSourceByName returns a OAuth2 LoginSource based on the given name
func GetActiveOAuth2LoginSourceByName(name string) (*Source, error) {
	loginSource := new(Source)
	has, err := db.GetEngine(db.DefaultContext).Where("name = ? and type = ? and is_active = ?", name, OAuth2, true).Get(loginSource)
	if !has || err != nil {
		return nil, err
	}

	return loginSource, nil
}
