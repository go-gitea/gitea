// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"slices"
	"strconv"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/optional"
)

func MakeSelfOnTop(doer *user.User, users []*user.User) []*user.User {
	if doer != nil {
		idx := slices.IndexFunc(users, func(u *user.User) bool {
			return u.ID == doer.ID
		})
		if idx > 0 {
			newUsers := make([]*user.User, len(users))
			newUsers[0] = users[idx]
			copy(newUsers[1:], users[:idx])
			copy(newUsers[idx+1:], users[idx+1:])
			return newUsers
		}
	}
	return users
}

// GetFilterUserIDByName tries to get the user ID from the given username.
// Before, the "issue filter" passes user ID to query the list, but in many cases, it's impossible to pre-fetch the full user list.
// So it's better to make it work like GitHub: users could input username directly.
// Since it only converts the username to ID directly and is only used internally (to search issues), so no permission check is needed.
// Return values:
// * nil: no filter
// * some(id): match the id, the id could be -1 to match the issues without assignee
// * some(NonExistingID): match no issue (due to the user doesn't exist)
func GetFilterUserIDByName(ctx context.Context, name string) optional.Option[int64] {
	if name == "" {
		return optional.None[int64]()
	}
	u, err := user.GetUserByName(ctx, name)
	if err != nil {
		if id, err := strconv.ParseInt(name, 10, 64); err == nil {
			return optional.Some(id)
		}
		return optional.Some(db.NonExistingID)
	}
	return optional.Some(u.ID)
}
