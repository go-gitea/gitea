// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
)

// IsUserOrgOwner returns true if user is in the owner team of given organization.
func IsUserOrgOwner(users user_model.UserList, orgID int64) map[int64]bool {
	results := make(map[int64]bool, len(users))
	for _, user := range users {
		results[user.ID] = false // Set default to false
	}
	ownerMaps, err := loadOrganizationOwners(db.GetEngine(db.DefaultContext), users, orgID)
	if err == nil {
		for _, owner := range ownerMaps {
			results[owner.UID] = true
		}
	}
	return results
}

func loadOrganizationOwners(e db.Engine, users user_model.UserList, orgID int64) (map[int64]*TeamUser, error) {
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

	userIDs := users.GetUserIDs()
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
