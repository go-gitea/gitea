// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/login"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
)

// UserList is a list of user.
// This type provide valuable methods to retrieve information for a group of users efficiently.
type UserList []*user_model.User

func (users UserList) getUserIDs() []int64 {
	userIDs := make([]int64, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID) // Considering that user id are unique in the list
	}
	return userIDs
}

// IsUserOrgOwner returns true if user is in the owner team of given organization.
func (users UserList) IsUserOrgOwner(orgID int64) map[int64]bool {
	results := make(map[int64]bool, len(users))
	for _, user := range users {
		results[user.ID] = false // Set default to false
	}
	ownerMaps, err := users.loadOrganizationOwners(db.GetEngine(db.DefaultContext), orgID)
	if err == nil {
		for _, owner := range ownerMaps {
			results[owner.UID] = true
		}
	}
	return results
}

func (users UserList) loadOrganizationOwners(e db.Engine, orgID int64) (map[int64]*TeamUser, error) {
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
		results[user.ID] = false // Set default to false
	}
	tokenMaps, err := users.loadTwoFactorStatus(db.GetEngine(db.DefaultContext))
	if err == nil {
		for _, token := range tokenMaps {
			results[token.UID] = true
		}
	}

	return results
}

func (users UserList) loadTwoFactorStatus(e db.Engine) (map[int64]*login.TwoFactor, error) {
	if len(users) == 0 {
		return nil, nil
	}

	userIDs := users.getUserIDs()
	tokenMaps := make(map[int64]*login.TwoFactor, len(userIDs))
	err := e.
		In("uid", userIDs).
		Find(&tokenMaps)
	if err != nil {
		return nil, fmt.Errorf("find two factor: %v", err)
	}
	return tokenMaps, nil
}

// GetUsersByIDs returns all resolved users from a list of Ids.
func GetUsersByIDs(ids []int64) (UserList, error) {
	ous := make([]*user_model.User, 0, len(ids))
	if len(ids) == 0 {
		return ous, nil
	}
	err := db.GetEngine(db.DefaultContext).In("id", ids).
		Asc("name").
		Find(&ous)
	return ous, err
}
