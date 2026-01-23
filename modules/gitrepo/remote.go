// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"errors"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	giturl "code.gitea.io/gitea/modules/git/url"
	"code.gitea.io/gitea/modules/globallock"
	"code.gitea.io/gitea/modules/util"
)

type RemoteOption string

const (
	RemoteOptionMirrorPush  RemoteOption = "--mirror=push"
	RemoteOptionMirrorFetch RemoteOption = "--mirror=fetch"
)

func GitRemoteAdd(ctx context.Context, repo Repository, remoteName, remoteURL string, options ...RemoteOption) error {
	return globallock.LockAndDo(ctx, getRepoConfigLockKey(repo.RelativePath()), func(ctx context.Context) error {
		cmd := gitcmd.NewCommand("remote", "add")
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
		_, _, err := RunCmdString(ctx, repo, cmd.AddDynamicArguments(remoteName, remoteURL))
		return err
	})
}

func GitRemoteRemove(ctx context.Context, repo Repository, remoteName string) error {
	return globallock.LockAndDo(ctx, getRepoConfigLockKey(repo.RelativePath()), func(ctx context.Context) error {
		cmd := gitcmd.NewCommand("remote", "rm").AddDynamicArguments(remoteName)
		_, _, err := RunCmdString(ctx, repo, cmd)
		return err
	})
}

// GitRemoteGetURL returns the url of a specific remote of the repository.
func GitRemoteGetURL(ctx context.Context, repo Repository, remoteName string) (*giturl.GitURL, error) {
	addr, err := git.GetRemoteAddress(ctx, repoPath(repo), remoteName)
	if err != nil {
		return nil, err
	}
	if addr == "" {
		return nil, util.NewNotExistErrorf("remote '%s' does not exist", remoteName)
	}
	return giturl.ParseGitURL(addr)
}
