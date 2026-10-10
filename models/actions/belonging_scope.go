// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import "gitea.dev/modules/setting"

// BelongingScope identifies the owner (user/org) or repository an Actions secret or variable belongs to.
type BelongingScope struct {
	OwnerID int64
	RepoID  int64
}

// Normalized drops OwnerID from a repo level scope, a repo level scope must have OwnerID 0.
func (s BelongingScope) Normalized() BelongingScope {
	if s.OwnerID != 0 && s.RepoID != 0 {
		setting.PanicInDevOrTesting("BelongingScope has both OwnerID %d and RepoID %d set", s.OwnerID, s.RepoID)
		s.OwnerID = 0
	}
	return s
}
