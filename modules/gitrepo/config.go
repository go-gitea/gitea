// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
)

// ManagedConfigAdd add a git configuration key to a specific value for the given repository.
func ManagedConfigAdd(ctx context.Context, repo git.RepositoryFacade, key, value string) error {
	return git.LockConfigAndDo(ctx, repo, func(ctx context.Context) error {
		_, _, err := gitcmd.NewCommand("config", "--add").
			AddDynamicArguments(key, value).WithRepo(repo).RunStdString(ctx)
		return err
	})
}

// ManagedConfigSet updates a git configuration key to a specific value for the given repository.
// If the key does not exist, it will be created.
// If the key exists, it will be updated to the new value.
func ManagedConfigSet(ctx context.Context, repo git.RepositoryFacade, key, value string) error {
	return git.LockConfigAndDo(ctx, repo, func(ctx context.Context) error {
		_, _, err := gitcmd.NewCommand("config").
			AddDynamicArguments(key, value).WithRepo(repo).RunStdString(ctx)
		return err
	})
}
