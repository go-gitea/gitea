// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"slices"

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
