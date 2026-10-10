// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

// BelongingScope identifies the owner (user/org) or repository an Actions secret or variable belongs to.
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
	if ownerID != 0 {
		return BelongingScopeOwner(ownerID)
	} else if repoID != 0 {
		return BelongingScopeRepo(repoID)
	}
	return BelongingScope{}
}

func (scope BelongingScope) GetOwnerRepoIDs() (ownerID, repoID int64) {
	return scope.ownerID, scope.repoID
}
