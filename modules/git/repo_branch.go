// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"code.gitea.io/gitea/modules/git/gitcmd"
)

// BranchPrefix base dir of the branch information file store on git
const BranchPrefix = "refs/heads/"

// AddRemote adds a new remote to repository.
func (repo *Repository) AddRemote(name, url string, fetch bool) error {
	cmd := gitcmd.NewCommand("remote", "add")
	if fetch {
		cmd.AddArguments("-f")
	}
	cmd.AddDynamicArguments(name, url)

	_, _, err := cmd.RunStdString(repo.Ctx, &gitcmd.RunOpts{Dir: repo.Path})
	return err
}
