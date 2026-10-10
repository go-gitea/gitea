// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import "gitea.dev/modules/setting"

// BelongingScope identifies the owner (user/org) or repository an Actions secret or variable belongs to.
// Both zero values mean "global scope".
type BelongingScope struct {
	ownerID int64
	repoID  int64
}

func BelongingScopeOwner(ownerID int64) BelongingScope {
	return BelongingScope{ownerID: ownerID}
}

func BelongingScopeRepo(repoID int64) BelongingScope {
	return BelongingScope{repoID: repoID}
}

func BelongingScopeOwnerOrRepo(ownerID, repoID int64) BelongingScope {
	// TODO: in the future, we should do more refactoring to avoid using this function
	// The caller should know whether it is an owner or repo scope, and call the corresponding function directly.
	if ownerID != 0 && repoID != 0 {
		setting.PanicInDevOrTesting("BelongingScopeOwnerOrRepo: both ownerID and repoID are non-zero")
	}
	if repoID != 0 {
		return BelongingScopeRepo(repoID)
	} else if ownerID != 0 {
		return BelongingScopeOwner(ownerID)
	}
	return BelongingScope{}
}

func (scope BelongingScope) GetOwnerRepoIDs() (ownerID, repoID int64) {
	return scope.ownerID, scope.repoID
}
