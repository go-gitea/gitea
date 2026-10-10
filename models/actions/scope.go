// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

// Scope identifies the owner (user/org) or repository an Actions secret or variable belongs to.
type Scope struct {
	OwnerID int64
	RepoID  int64
}

// Normalized drops OwnerID when RepoID is set, a repo level scope must have OwnerID 0.
func (s Scope) Normalized() Scope {
	if s.RepoID != 0 {
		s.OwnerID = 0
	}
	return s
}
