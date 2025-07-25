// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"io"
	"time"

	"code.gitea.io/gitea/modules/git"
	giturl "code.gitea.io/gitea/modules/git/url"
	"code.gitea.io/gitea/modules/globallock"
)

func GitConfigGet(ctx context.Context, repo Repository, key string) (string, error) {
	result, _, err := git.NewCommand("config", "--get").
		AddDynamicArguments(key).
		RunStdString(ctx, &git.RunOpts{Dir: repoPath(repo)})
	if err != nil {
		return "", err
	}
	if len(result) > 0 {
		result = result[:len(result)-1] // remove trailing newline
	}
	return result, nil
}

func repoGitConfigLockKey(repoStoragePath string) string {
	return "repo-config:" + repoStoragePath
}

// GitConfigAdd add a git configuration key to a specific value for the given repository.
func GitConfigAdd(ctx context.Context, repo Repository, key, value string) error {
	releaser, err := globallock.Lock(ctx, repoGitConfigLockKey(repo.RelativePath()))
	if err != nil {
		return err
	}
	defer releaser()

	_, _, err = git.NewCommand("config", "--add").
		AddDynamicArguments(key, value).
		RunStdString(ctx, &git.RunOpts{Dir: repoPath(repo)})
	return err
}

// GitConfigSet updates a git configuration key to a specific value for the given repository.
// If the key does not exist, it will be created.
// If the key exists, it will be updated to the new value.
func GitConfigSet(ctx context.Context, repo Repository, key, value string) error {
	releaser, err := globallock.Lock(ctx, repoGitConfigLockKey(repo.RelativePath()))
	if err != nil {
		return err
	}
	defer releaser()

	_, _, err = git.NewCommand("config").
		AddDynamicArguments(key, value).
		RunStdString(ctx, &git.RunOpts{Dir: repoPath(repo)})
	return err
}

func GitRemoteAdd(ctx context.Context, repo Repository, remoteName, remoteURL string, options ...string) error {
	releaser, err := globallock.Lock(ctx, repoGitConfigLockKey(repo.RelativePath()))
	if err != nil {
		return err
	}
	defer releaser()

	cmd := git.NewCommand("remote", "add")
	if len(options) > 0 {
		cmd.AddDynamicArguments(options...)
	}
	_, _, err = cmd.
		AddDynamicArguments(remoteName, remoteURL).
		RunStdString(ctx, &git.RunOpts{Dir: repoPath(repo)})
	return err
}

func GitRemoteRemove(ctx context.Context, repo Repository, remoteName string) error {
	releaser, err := globallock.Lock(ctx, repoGitConfigLockKey(repo.RelativePath()))
	if err != nil {
		return err
	}
	defer releaser()

	cmd := git.NewCommand("remote", "rm").AddDynamicArguments(remoteName)
	_, _, err = cmd.RunStdString(ctx, &git.RunOpts{Dir: repoPath(repo)})
	return err
}

// GitRemoteGetURL returns the url of a specific remote of the repository.
func GitRemoteGetURL(ctx context.Context, repo Repository, remoteName string) (*giturl.GitURL, error) {
	addr, err := git.GetRemoteAddress(ctx, repoPath(repo), remoteName)
	if err != nil {
		return nil, err
	}
	return giturl.ParseGitURL(addr)
}

// FIXME: config related? long-time running?
func GitRemotePrune(ctx context.Context, repo Repository, remoteName string, timeout time.Duration, stdout, stderr io.Writer) error {
	releaser, err := globallock.Lock(ctx, repoGitConfigLockKey(repo.RelativePath()))
	if err != nil {
		return err
	}
	defer releaser()

	return git.NewCommand("remote", "prune").AddDynamicArguments(remoteName).
		Run(ctx, &git.RunOpts{
			Timeout: timeout,
			Dir:     repoPath(repo),
			Stdout:  stdout,
			Stderr:  stderr,
		})
}

// FIXME: config related? long-time running?
func GitRemoteUpdatePrune(ctx context.Context, repo Repository, remoteName string, timeout time.Duration, stdout, stderr io.Writer) error {
	releaser, err := globallock.Lock(ctx, repoGitConfigLockKey(repo.RelativePath()))
	if err != nil {
		return err
	}
	defer releaser()

	return git.NewCommand("remote", "update", "--prune").AddDynamicArguments(remoteName).
		Run(ctx, &git.RunOpts{
			Timeout: timeout,
			Dir:     repoPath(repo),
			Stdout:  stdout,
			Stderr:  stderr,
		})
}
