// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
)

//UserList is a list of user.
// This type provide valuable methods to retrieve information for a group of users efficiently.
type UserList []*User

func (users UserList) getUserIDs() []int64 {
	userIDs := make([]int64, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID) //Considering that user id are unique in the list
	}
	return userIDs
}

// IsUserOrgOwner returns true if user is in the owner team of given organization.
func (users UserList) IsUserOrgOwner(orgID int64) map[int64]bool {
	results := make(map[int64]bool, len(users))
	//TODO use directly xorm
	for _, u := range users {
		results[u.ID] = u.IsUserOrgOwner(orgID)
	}
	return results
}

// GetTwoFaStatus return state of 2FA enrollement
func (users UserList) GetTwoFaStatus() map[int64]bool {
	results := make(map[int64]bool, len(users))
	for _, user := range users {
		results[user.ID] = false //Set default to false
	}
	tokenMaps, err := users.loadTwoFactorStatus(x)
	if err == nil {
		for _, token := range tokenMaps {
			results[token.UID] = true
		}
	}

	return results
}

func (users UserList) loadTwoFactorStatus(e Engine) (map[int64]*TwoFactor, error) {
	if len(users) == 0 {
		return nil, nil
	}

	userIDs := users.getUserIDs()
	tokenMaps := make(map[int64]*TwoFactor, len(userIDs))
	err := e.
		In("uid", userIDs).
		Find(&tokenMaps)
	if err != nil {
		return nil, fmt.Errorf("find two factor: %v", err)
	}
	return tokenMaps, nil
}
