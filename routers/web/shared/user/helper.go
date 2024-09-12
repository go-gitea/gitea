// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"sort"

	"code.gitea.io/gitea/models/user"
)

func MakeSelfOnTop(doer *user.User, users []*user.User) []*user.User {
	if doer != nil {
		sort.Slice(users, func(i, j int) bool {
			if users[i].ID == users[j].ID {
				return false
			}
			return users[i].ID == doer.ID // if users[i] is self, put it before others, so less=true
		})
	}
	return users
}
