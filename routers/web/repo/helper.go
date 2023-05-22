// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"sort"

	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
)

func makeSelfOnTop(ctx *context.Context, users []*user.User) []*user.User {
	if ctx.Doer != nil {
		sort.Slice(users, func(i, j int) bool {
			if users[i].ID == users[j].ID {
				return false
			}
			return users[i].ID == ctx.Doer.ID // if users[i] is self, put it before others, so less=true
		})
	}
	return users
}
