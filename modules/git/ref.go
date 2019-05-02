// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

// Reference represents a Git ref.
type Reference struct {
	Name   string
	repo   *Repository
	Object SHA1 // The id of this commit object
	Type   string
}

// Commit return the commit of the reference
func (ref *Reference) Commit() (*Commit, error) {
	return ref.repo.getCommit(ref.Object)
}
