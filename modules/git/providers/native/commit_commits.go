// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"container/list"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
)

//  _
// /   _  ._ _  ._ _  o _|_
// \_ (_) | | | | | | |  |_
//

// CommitsCount returns number of total commits of until current revision.
func (c *Commit) CommitsCount() (int64, error) {
	return git.CommitsCount(c.Repository().Path(), c.ID().String())
}

// CommitsByRange returns the specific page commits before current revision, every page's number default by CommitsRangeSize
func (c *Commit) CommitsByRange(page, pageSize int) (*list.List, error) {
	return gitService.CommitsByFileAndRange(c.Repository(), c.ID().String(), "", page, pageSize)
}

// CommitsBefore returns all the commits before current revision
func (c *Commit) CommitsBefore() (*list.List, error) {
	return gitService.CommitsBefore(c.Repository(), c.ID().String(), 0)
}

// HasPreviousCommit returns true if a given commitHash is contained in commit's parents
func (c *Commit) HasPreviousCommit(commitHash service.Hash) (bool, error) {
	for i := 0; i < c.ParentCount(); i++ {
		commit, err := c.Parent(i)
		if err != nil {
			return false, err
		}
		if commit.ID().String() == commitHash.String() {
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

// CommitsBeforeLimit returns num commits before current revision
func (c *Commit) CommitsBeforeLimit(num int) (*list.List, error) {
	return gitService.CommitsBefore(c.Repository(), c.ID().String(), num)
}

// CommitsBeforeUntil returns the commits between commitID to current revision
func (c *Commit) CommitsBeforeUntil(commitID string) (*list.List, error) {
	endCommit, err := c.Repository().GetCommit(commitID)
	if err != nil {
		return nil, err
	}
	return gitService.CommitsBetween(c.Repository(), c, endCommit)
}

// SearchCommits returns the commits match the keyword before current revision
func (c *Commit) SearchCommits(opts service.SearchCommitsOptions) (*list.List, error) {
	return gitService.SearchCommits(c.Repository(), c.ID().String(), opts)
}
