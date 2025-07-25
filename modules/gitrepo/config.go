// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"errors"
	"io"
	"time"

	"code.gitea.io/gitea/modules/git"
	giturl "code.gitea.io/gitea/modules/git/url"
	"code.gitea.io/gitea/modules/globallock"
)

func GetGitConfig(ctx context.Context, repo Repository, key string) (string, error) {
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

func getRepoConfigLockKey(repoStoragePath string) string {
	return "repo-config:" + repoStoragePath
}

// AddGitConfig add a git configuration key to a specific value for the given repository.
func AddGitConfig(ctx context.Context, repo Repository, key, value string) error {
	releaser, err := globallock.Lock(ctx, getRepoConfigLockKey(repo.RelativePath()))
	if err != nil {
		return err
	}
	defer releaser()

	_, _, err = git.NewCommand("config", "--add").
		AddDynamicArguments(key, value).
		RunStdString(ctx, &git.RunOpts{Dir: repoPath(repo)})
	return err
}

// UpdateGitConfig updates a git configuration key to a specific value for the given repository.
// If the key does not exist, it will be created.
// If the key exists, it will be updated to the new value.
func UpdateGitConfig(ctx context.Context, repo Repository, key, value string) (string, error) {
	releaser, err := globallock.Lock(ctx, getRepoConfigLockKey(repo.RelativePath()))
	if err != nil {
		return "", err
	}
	defer releaser()

	value, _, err1 := git.NewCommand("config").
		AddDynamicArguments(key, value).
		RunStdString(ctx, &git.RunOpts{Dir: repoPath(repo)})
	return value, err1
}

type RemoteOption string

const (
	RemoteOptionMirrorPush  RemoteOption = "--mirror=push"
	RemoteOptionMirrorFetch RemoteOption = "--mirror=fetch"
)

func AddGitRemote(ctx context.Context, repo Repository, remoteName, remoteURL string, options ...RemoteOption) error {
	releaser, err := globallock.Lock(ctx, getRepoConfigLockKey(repo.RelativePath()))
	if err != nil {
		return err
	}
	defer releaser()

	cmd := git.NewCommand("remote", "add")
	if len(options) > 0 {
		switch options[0] {
		case RemoteOptionMirrorPush:
			cmd.AddArguments("--mirror=push")
		case RemoteOptionMirrorFetch:
			cmd.AddArguments("--mirror=fetch")
		default:
			return errors.New("unknown remote option: " + string(options[0]))
		}
	}
	_, _, err = cmd.
		AddDynamicArguments(remoteName, remoteURL).
		RunStdString(ctx, &git.RunOpts{Dir: repoPath(repo)})
	return err
}

func RemoveGitRemote(ctx context.Context, repo Repository, remoteName string) error {
	releaser, err := globallock.Lock(ctx, getRepoConfigLockKey(repo.RelativePath()))
	if err != nil {
		return err
	}
	defer releaser()

	cmd := git.NewCommand("remote", "rm").AddDynamicArguments(remoteName)
	_, _, err = cmd.RunStdString(ctx, &git.RunOpts{Dir: repoPath(repo)})
	return err
}

// GetRemoteURL returns the url of a specific remote of the repository.
func GetRemoteURL(ctx context.Context, repo Repository, remoteName string) (*giturl.GitURL, error) {
	addr, err := git.GetRemoteAddress(ctx, repoPath(repo), remoteName)
	if err != nil {
		return nil, err
	}
	if addr == "" {
		return nil, nil
	}
	return giturl.ParseGitURL(addr)
}

func SetRemoteURL(ctx context.Context, repo Repository, remoteName, remoteURL string) error {
	releaser, err := globallock.Lock(ctx, getRepoConfigLockKey(repo.RelativePath()))
	if err != nil {
		return err
	}
	defer releaser()

	cmd := git.NewCommand("remote", "set-url").AddDynamicArguments(remoteName, remoteURL)
	_, _, err = cmd.RunStdString(ctx, &git.RunOpts{Dir: repoPath(repo)})
	return err
}

// PruneRemote prunes the remote branches that no longer exist in the remote repository.
// No lock is needed because the remote remoteName will be checked before invoking this function.
// Then it will not update the remote automatically if the remote does not exist.
func PruneRemote(ctx context.Context, repo Repository, remoteName string, timeout time.Duration, stdout, stderr io.Writer) error {
	return git.NewCommand("remote", "prune").AddDynamicArguments(remoteName).
		Run(ctx, &git.RunOpts{
			Timeout: timeout,
			Dir:     repoPath(repo),
			Stdout:  stdout,
			Stderr:  stderr,
		})
}

// UpdateRemotePrune updates the remote branches and prunes the ones that no longer exist in the remote repository.
// No lock is needed because the remote remoteName will be checked before invoking this function.
// Then it will not update the remote automatically if the remote does not exist.
func UpdateRemotePrune(ctx context.Context, repo Repository, remoteName string, timeout time.Duration, stdout, stderr io.Writer) error {
	return git.NewCommand("remote", "update", "--prune").AddDynamicArguments(remoteName).
		Run(ctx, &git.RunOpts{
			Timeout: timeout,
			Dir:     repoPath(repo),
			Stdout:  stdout,
			Stderr:  stderr,
		})
}
