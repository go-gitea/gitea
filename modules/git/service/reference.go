// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

// Reference represents a Git ref.
type Reference interface {
	// Name returns the name of this reference
	Name() string

	// ID returns the Hash for this reference
	ID() Hash

	// Type returns the git object type of this reference
	Type() ObjectType

	// ShortName returns the short name of the reference
	ShortName() string

	// RefGroup returns the group type of the reference
	RefGroup() string

	// Commit return the commit of the reference
	Commit() (Commit, error)
}
