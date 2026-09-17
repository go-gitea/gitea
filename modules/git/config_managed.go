// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"strings"

	"gitea.dev/modules/git/gitcmd"
)

// ManagedConfigAdd add a git configuration key to a specific value for the given repository.
func ManagedConfigAdd(ctx context.Context, repo RepositoryFacade, key, value string) error {
	return LockConfigAndDo(ctx, repo, func(ctx context.Context) error {
		_, _, err := gitcmd.NewCommand("config", "--add").
			AddDynamicArguments(key, value).WithRepo(repo).RunStdString(ctx)
		return err
	})
}

// ManagedConfigSet updates a git configuration key to a specific value for the given repository.
// If the key does not exist, it will be created.
// If the key exists, it will be updated to the new value.
func ManagedConfigSet(ctx context.Context, repo RepositoryFacade, key, value string) error {
	return LockConfigAndDo(ctx, repo, func(ctx context.Context) error {
		_, _, err := gitcmd.NewCommand("config").
			AddDynamicArguments(key, value).WithRepo(repo).RunStdString(ctx)
		return err
	})
}

// ManagedConfigGetRegexp returns the git configuration entries of the given repository whose key matches
// keyRegexp, as a map of key to its (possibly multiple) values.
func ManagedConfigGetRegexp(ctx context.Context, repo RepositoryFacade, keyRegexp string) (map[string][]string, error) {
	stdout, _, err := gitcmd.NewCommand("config", "--get-regexp").
		AddDynamicArguments(keyRegexp).WithRepo(repo).RunStdString(ctx)
	res := map[string][]string{}
	if gitcmd.IsErrorExitCode(err, 1) {
		return res, nil // no match
	} else if err != nil {
		return nil, err
	}

	for line := range strings.SplitSeq(strings.TrimRight(stdout, "\r\n"), "\n") {
		key, value, _ := strings.Cut(strings.TrimRight(line, "\r"), " ")
		if key != "" {
			res[key] = append(res[key], value)
		}
	}
	return res, nil
}

// ManagedConfigUnsetAll removes all values of a git configuration key from the given repository.
func ManagedConfigUnsetAll(ctx context.Context, repo RepositoryFacade, key string) error {
	return LockConfigAndDo(ctx, repo, func(ctx context.Context) error {
		_, _, err := gitcmd.NewCommand("config", "--unset-all").
			AddDynamicArguments(key).WithRepo(repo).RunStdString(ctx)
		if gitcmd.IsErrorExitCode(err, 5) {
			return nil // the key does not exist
		}
		return err
	})
}
