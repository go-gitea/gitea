// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"slices"
	"strconv"

	"code.gitea.io/gitea/models/user"
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
// Old usage: poster=123, new usage: poster=the-username (at the moment, non-existing username is treated as poster=0, not ideal but acceptable)
func GetFilterUserIDByName(ctx context.Context, name string) int64 {
	if name == "" {
		return 0
	}
	u, err := user.GetUserByName(ctx, name)
	if err != nil {
		if id, err := strconv.ParseInt(name, 10, 64); err == nil {
			return id
		}
		return 0
	}
	return u.ID
}
