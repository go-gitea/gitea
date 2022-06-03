// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	"context"
	"fmt"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/util"
)

// AddPushMirrorRemote registers the push mirror remote.
func AddPushMirrorRemote(ctx context.Context, m *repo_model.PushMirror, addr string) error {
	addRemoteAndConfig := func(addr, path string) error {
		cmd := git.NewCommand(ctx, "remote", "add", "--mirror=push", m.RemoteName, addr)
		if strings.Contains(addr, "://") && strings.Contains(addr, "@") {
			cmd.SetDescription(fmt.Sprintf("remote add %s --mirror=push %s [repo_path: %s]", m.RemoteName, util.SanitizeCredentialURLs(addr), path))
		} else {
			cmd.SetDescription(fmt.Sprintf("remote add %s --mirror=push %s [repo_path: %s]", m.RemoteName, addr, path))
		}
		if _, _, err := cmd.RunStdString(&git.RunOpts{Dir: path}); err != nil {
			return err
		}
		if _, _, err := git.NewCommand(ctx, "config", "--add", "remote."+m.RemoteName+".push", "+refs/heads/*:refs/heads/*").RunStdString(&git.RunOpts{Dir: path}); err != nil {
			return err
		}
		if _, _, err := git.NewCommand(ctx, "config", "--add", "remote."+m.RemoteName+".push", "+refs/tags/*:refs/tags/*").RunStdString(&git.RunOpts{Dir: path}); err != nil {
			return err
		}
		return nil
	}

	if err := addRemoteAndConfig(addr, m.Repo.RepoPath()); err != nil {
		return err
	}

	if m.Repo.HasWiki() {
		wikiRemoteURL := repository.WikiRemoteURL(ctx, addr)
		if len(wikiRemoteURL) > 0 {
			if err := addRemoteAndConfig(wikiRemoteURL, m.Repo.WikiPath()); err != nil {
				return err
			}
		}
	}

	return nil
}

// RemovePushMirrorRemote removes the push mirror remote.
func RemovePushMirrorRemote(ctx context.Context, m *repo_model.PushMirror) error {
	cmd := git.NewCommand(ctx, "remote", "rm", m.RemoteName)
	_ = m.GetRepository()

	if _, _, err := cmd.RunStdString(&git.RunOpts{Dir: m.Repo.RepoPath()}); err != nil {
		return err
	}

	if m.Repo.HasWiki() {
		if _, _, err := cmd.RunStdString(&git.RunOpts{Dir: m.Repo.WikiPath()}); err != nil {
			// The wiki remote may not exist
			log.Warn("Wiki Remote[%d] could not be removed: %v", m.ID, err)
		}
	}

	return nil
}
