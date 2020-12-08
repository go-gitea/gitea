// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

// Branch represents a branch from a repository
// FIXME: This doesn't seem to be a useful entity and could be removed
type Branch interface {
	// Name returns the branch name
	Name() string

	// GetCommit returns the head commit of a branch
	GetCommit() (Commit, error)
}

// DeleteBranchOptions Option(s) for delete branch
type DeleteBranchOptions struct {
	Force bool
}
