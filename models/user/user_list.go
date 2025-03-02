// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"

	"code.gitea.io/gitea/models/db"
)

func GetUsersMapByIDs(ctx context.Context, userIDs []int64) (map[int64]*User, error) {
	userMaps := make(map[int64]*User, len(userIDs))
	left := len(userIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		err := db.GetEngine(ctx).
			In("id", userIDs[:limit]).
			Find(&userMaps)
		if err != nil {
			return nil, err
		}
		left -= limit
		userIDs = userIDs[limit:]
	}
	return userMaps, nil
}

func GetPossibleUserFromMap(userID int64, usererMaps map[int64]*User) *User {
	switch userID {
	case GhostUserID:
		return NewGhostUser()
	case ActionsUserID:
		return NewActionsUser()
	case 0:
		return nil
	default:
		user, ok := usererMaps[userID]
		if !ok {
			return NewGhostUser()
		}
		return user
	}
}
