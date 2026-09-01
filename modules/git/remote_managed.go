// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"

	"gitea.dev/modules/git/gitcmd"
	giturl "gitea.dev/modules/git/url"
	"gitea.dev/modules/util"
)

type RemoteOption string

const (
	RemoteOptionMirrorPush  RemoteOption = "--mirror=push"
	RemoteOptionMirrorFetch RemoteOption = "--mirror=fetch"
)

func ManagedRemoteAdd(ctx context.Context, repo RepositoryFacade, remoteName, remoteURL string, options ...RemoteOption) error {
	return LockConfigAndDo(ctx, repo, func(ctx context.Context) error {
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
		_, _, err := cmd.AddDynamicArguments(remoteName, remoteURL).WithRepo(repo).RunStdString(ctx)
		return err
	})
}

func ManagedRemoteRemove(ctx context.Context, repo RepositoryFacade, remoteName string) error {
	return LockConfigAndDo(ctx, repo, func(ctx context.Context) error {
		cmd := gitcmd.NewCommand("remote", "rm").AddDynamicArguments(remoteName)
		_, _, err := cmd.WithRepo(repo).RunStdString(ctx)
		return err
	})
}

func ParseRemoteAddressURL(ctx context.Context, repo RepositoryFacade, remoteName string) (*giturl.GitURL, error) {
	addr, err := GetRemoteAddress(ctx, repo, remoteName)
	if err != nil {
		return nil, err
	}
	if addr == "" {
		return nil, util.NewNotExistErrorf("remote '%s' does not exist", remoteName)
	}
	return giturl.ParseGitURL(addr)
}
