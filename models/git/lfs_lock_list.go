// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
)

// LFSLockList is a list of LFSLock
type LFSLockList []*LFSLock

// LoadAttributes loads the attributes for the given locks
func (locks LFSLockList) LoadAttributes(ctx context.Context) error {
	if len(locks) == 0 {
		return nil
	}

	if err := locks.LoadOwner(ctx); err != nil {
		return fmt.Errorf("load owner: %w", err)
	}

	return nil
}

// LoadOwner loads the owner of the locks
func (locks LFSLockList) LoadOwner(ctx context.Context) error {
	if len(locks) == 0 {
		return nil
	}

	usersIDs := container.FilterSlice(locks, func(lock *LFSLock) (int64, bool) {
		return lock.OwnerID, true
	})
	users := make(map[int64]*user_model.User, len(usersIDs))
	if err := db.GetEngine(ctx).
		In("id", usersIDs).
		Find(&users); err != nil {
		return fmt.Errorf("find users: %w", err)
	}
	for _, v := range locks {
		v.Owner = users[v.OwnerID]
		if v.Owner == nil { // not exist
			v.Owner = user_model.NewGhostUser()
		}
	}

	return nil
}
