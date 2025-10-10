// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// BranchPrefix base dir of the branch information file store on git
const BranchPrefix = "refs/heads/"

// AddRemote adds a new remote to repository.
func (repo *Repository) AddRemote(ctx context.Context, name, url string, fetch bool) error {
	cmd := gitcmd.NewCommand("remote", "add")
	if fetch {
		cmd.AddArguments("-f")
	}
	_, _, err := cmd.AddDynamicArguments(name, url).
		WithDir(repo.Path).
		RunStdString(ctx)
	return err
}
