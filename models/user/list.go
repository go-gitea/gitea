// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"fmt"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
)

// UserList is a list of user.
// This type provide valuable methods to retrieve information for a group of users efficiently.
type UserList []*User //revive:disable-line:exported

// GetUserIDs returns a slice of user's id
func (users UserList) GetUserIDs() []int64 {
	userIDs := make([]int64, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID) // Considering that user id are unique in the list
	}
	return userIDs
}

// GetTwoFaStatus return state of 2FA enrollement
func (users UserList) GetTwoFaStatus() map[int64]bool {
	results := make(map[int64]bool, len(users))
	for _, user := range users {
		results[user.ID] = false // Set default to false
	}

	if tokenMaps, err := users.loadTwoFactorStatus(db.GetEngine(db.DefaultContext)); err == nil {
		for _, token := range tokenMaps {
			results[token.UID] = true
		}
	}

	if credentialMaps, err := users.loadWebAuthnStatus(db.GetEngine(db.DefaultContext)); err == nil {
		for _, cred := range credentialMaps {
			results[cred.UserID] = true
		}
	}

	return results
}

func (users UserList) loadTwoFactorStatus(e db.Engine) (map[int64]*auth.TwoFactor, error) {
	if len(users) == 0 {
		return nil, nil
	}

	userIDs := users.GetUserIDs()
	tokenMaps := make(map[int64]*auth.TwoFactor, len(userIDs))
	if err := e.In("uid", userIDs).Find(&tokenMaps); err != nil {
		return nil, fmt.Errorf("find two factor: %v", err)
	}
	return tokenMaps, nil
}

func (users UserList) loadWebAuthnStatus(e db.Engine) (map[int64]*auth.WebAuthnCredential, error) {
	if len(users) == 0 {
		return nil, nil
	}

	userIDs := users.GetUserIDs()
	credentialMaps := make(map[int64]*auth.WebAuthnCredential, len(userIDs))
	if err := e.In("user_id", userIDs).Find(&credentialMaps); err != nil {
		return nil, fmt.Errorf("find two factor: %v", err)
	}
	return credentialMaps, nil
}

// GetUsersByIDs returns all resolved users from a list of Ids.
func GetUsersByIDs(ids []int64) (UserList, error) {
	ous := make([]*User, 0, len(ids))
	if len(ids) == 0 {
		return ous, nil
	}
	err := db.GetEngine(db.DefaultContext).In("id", ids).
		Asc("name").
		Find(&ous)
	return ous, err
}
