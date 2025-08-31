// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/globallock"
)

func GitConfigGet(ctx context.Context, repo Repository, key string) (string, error) {
	result, _, err := git.NewCommand("config", "--get").
		AddDynamicArguments(key).
		RunStdString(ctx, &git.RunOpts{Dir: repoPath(repo)})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result), nil
}

func getRepoConfigLockKey(repoStoragePath string) string {
	return "repo-config:" + repoStoragePath
}

// GitConfigAdd add a git configuration key to a specific value for the given repository.
func GitConfigAdd(ctx context.Context, repo Repository, key, value string) error {
	return globallock.LockAndDo(ctx, getRepoConfigLockKey(repo.RelativePath()), func(ctx context.Context) error {
		_, _, err := git.NewCommand("config", "--add").
			AddDynamicArguments(key, value).
			RunStdString(ctx, &git.RunOpts{Dir: repoPath(repo)})
		return err
	})
}

// GitConfigSet updates a git configuration key to a specific value for the given repository.
// If the key does not exist, it will be created.
// If the key exists, it will be updated to the new value.
func GitConfigSet(ctx context.Context, repo Repository, key, value string) error {
	return globallock.LockAndDo(ctx, getRepoConfigLockKey(repo.RelativePath()), func(ctx context.Context) error {
		_, _, err := git.NewCommand("config").
			AddDynamicArguments(key, value).
			RunStdString(ctx, &git.RunOpts{Dir: repoPath(repo)})
		return err
	})
}
