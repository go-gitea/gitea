// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

// BelongingScope identifies the owner (user/org) or repository an Actions secret or variable belongs to.
// Both zero values mean "global scope".
// Always use the functions to create a BelongingScope, rather than creating one directly, to avoid accidentally creating an invalid scope.
type BelongingScope struct {
	internalOwnerID int64
	iternalRepoID   int64
}

func BelongingScopeOwner(ownerID int64) BelongingScope {
	return BelongingScope{internalOwnerID: ownerID}
}

func BelongingScopeRepo(repoID int64) BelongingScope {
	return BelongingScope{iternalRepoID: repoID}
}

func BelongingScopeGlobal() BelongingScope {
	return BelongingScope{}
}

func (scope BelongingScope) GetOwnerRepoIDs() (ownerID, repoID int64) {
	return scope.internalOwnerID, scope.iternalRepoID
}
