// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git"
)

//  _
// |_) ._  _. ._   _ |_
// |_) |  (_| | | (_ | |
//

// Branch returns the branch that this commit is on
func (commit *Commit) Branch() string {
	return commit.branch
}

// GetBranchName gets the closest branch name (as returned by 'git name-rev --name-only')
func (commit *Commit) GetBranchName() (string, error) {
	err := git.LoadGitVersion()
	if err != nil {
		return "", fmt.Errorf("Git version missing: %v", err)
	}

	args := []string{
		"name-rev",
	}
	if git.CheckGitVersionAtLeast("2.13.0") == nil {
		args = append(args, "--exclude", "refs/tags/*")
	}
	args = append(args, "--name-only", "--no-undefined", commit.ID().String())

	data, err := git.NewCommand(args...).RunInDir(commit.Repository().Path())
	if err != nil {
		// handle special case where git can not describe commit
		if strings.Contains(err.Error(), "cannot describe") {
			return "", nil
		}

		return "", err
	}

	// name-rev commitID output will be "master" or "master~12"
	return strings.SplitN(strings.TrimSpace(data), "~", 2)[0], nil
}

// LoadBranchName load branch name for commit
func (commit *Commit) LoadBranchName() (err error) {
	if len(commit.branch) != 0 {
		return
	}

	commit.branch, err = commit.GetBranchName()
	return
}

// GetTagName gets the current tag name for given commit
func (commit *Commit) GetTagName() (string, error) {
	data, err := git.NewCommand("describe", "--exact-match", "--tags", "--always", commit.ID().String()).RunInDir(commit.Repository().Path())
	if err != nil {
		// handle special case where there is no tag for this commit
		if strings.Contains(err.Error(), "no tag exactly matches") {
			return "", nil
		}

		return "", err
	}

	return strings.TrimSpace(data), nil
}
