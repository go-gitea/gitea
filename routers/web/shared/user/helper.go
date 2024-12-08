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
