// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build gogit

package git

// HasPreviousCommit returns true if a given commitHash is contained in commit's parents
func (c *Commit) HasPreviousCommit(commitHash SHA1) (bool, error) {
	for i := 0; i < c.ParentCount(); i++ {
		commit, err := c.Parent(i)
		if err != nil {
			return false, err
		}
		if commit.ID == commitHash {
			return true, nil
		}
		commitInParentCommit, err := commit.HasPreviousCommit(commitHash)
		if err != nil {
			return false, err
		}
		if commitInParentCommit {
			return true, nil
		}
	}
	return false, nil
}
