// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
)

type BuildList []*Build

// GetUserIDs returns a slice of user's id
func (builds BuildList) GetUserIDs() []int64 {
	userIDsMap := make(map[int64]struct{})
	for _, build := range builds {
		userIDsMap[build.TriggerUserID] = struct{}{}
	}
	userIDs := make([]int64, 0, len(userIDsMap))
	for userID := range userIDsMap {
		userIDs = append(userIDs, userID)
	}
	return userIDs
}

func (builds BuildList) LoadTriggerUser() error {
	userIDs := builds.GetUserIDs()
	users := make(map[int64]*user_model.User, len(userIDs))
	if err := db.GetEngine(db.DefaultContext).In("id", userIDs).Find(&users); err != nil {
		return err
	}
	for _, task := range builds {
		task.TriggerUser = users[task.TriggerUserID]
	}
	return nil
}
