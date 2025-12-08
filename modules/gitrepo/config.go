// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/globallock"
)

func GitConfigGet(ctx context.Context, repo Repository, key string) (string, error) {
	result, err := RunCmdString(ctx, repo, gitcmd.NewCommand("config", "--get").
		AddDynamicArguments(key))
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
		_, err := RunCmdString(ctx, repo, gitcmd.NewCommand("config", "--add").
			AddDynamicArguments(key, value))
		return err
	})
}

// GitConfigSet updates a git configuration key to a specific value for the given repository.
// If the key does not exist, it will be created.
// If the key exists, it will be updated to the new value.
func GitConfigSet(ctx context.Context, repo Repository, key, value string) error {
	return globallock.LockAndDo(ctx, getRepoConfigLockKey(repo.RelativePath()), func(ctx context.Context) error {
		_, err := RunCmdString(ctx, repo, gitcmd.NewCommand("config").
			AddDynamicArguments(key, value))
		return err
	})
}
