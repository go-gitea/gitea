// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
)

// ToPushMirror convert from repo_model.PushMirror and remoteAddress to api.TopicResponse
func ToPushMirror(pm *repo_model.PushMirror, repo *repo_model.Repository) *api.PushMirror {
	remoteAddress, _ := getMirrorRemoteAddress(repo, pm.RemoteName)
	return &api.PushMirror{
		ID:             pm.ID,
		RepoName:       repo.Name,
		RemoteName:     pm.RemoteName,
		RemoteAddress:  remoteAddress,
		CreatedUnix:    pm.CreatedUnix.FormatLong(),
		LastUpdateUnix: pm.LastUpdateUnix.FormatLong(),
		LastError:      pm.LastError,
		Interval:       pm.Interval.String(),
	}
}

func getMirrorRemoteAddress(repo *repo_model.Repository, remoteName string) (string, error) {
	u, err := git.GetRemoteAddress(git.DefaultContext, repo.RepoPath(), remoteName)
	if err != nil {
		return "", err
	}
	// remove confidential information
	u.User = nil
	return u.String(), nil
}
