// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import "fmt"

// BelongingScope identifies the owner (user/org) or repository an Actions secret or variable belongs to.
type BelongingScope struct {
	OwnerID int64
	RepoID  int64
}

// AssertValid panics if both IDs are set, a repo level scope must have OwnerID 0.
func (s BelongingScope) AssertValid() {
	if s.OwnerID != 0 && s.RepoID != 0 {
		panic(fmt.Sprintf("BelongingScope has both OwnerID %d and RepoID %d set", s.OwnerID, s.RepoID))
	}
}
