// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
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
	for _, user := range users {
		results[user.ID] = false //Set default to false
	}
	ownerMaps, err := users.loadOrganizationOwners(x, orgID)
	if err == nil {
		for _, owner := range ownerMaps {
			results[owner.UID] = true
		}
	}
	return results
}

func (users UserList) loadOrganizationOwners(e Engine, orgID int64) (map[int64]*TeamUser, error) {
	if len(users) == 0 {
		return nil, nil
	}
	ownerTeam, err := getOwnerTeam(e, orgID)
	if err != nil {
		if IsErrTeamNotExist(err) {
			log.Error("Organization does not have owner team: %d", orgID)
			return nil, nil
		}
		return nil, err
	}

	userIDs := users.getUserIDs()
	ownerMaps := make(map[int64]*TeamUser)
	err = e.In("uid", userIDs).
		And("org_id=?", orgID).
		And("team_id=?", ownerTeam.ID).
		Find(&ownerMaps)
	if err != nil {
		return nil, fmt.Errorf("find team users: %v", err)
	}
	return ownerMaps, nil
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

//APIFormat return list of users in api format
func (users UserList) APIFormat() []*api.User {
	var result []*api.User
	for _, u := range users {
		result = append(result, u.APIFormat())
	}
	return result
}
