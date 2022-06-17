// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"errors"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
)

// ToPushMirror convert from repo_model.PushMirror and remoteAddress to api.TopicResponse
func ToPushMirror(pm *repo_model.PushMirror, repo *repo_model.Repository) (*api.PushMirror, error) {
	remoteAddress, err := getMirrorRemoteAddress(repo, pm.RemoteName)
	if err != nil {
		return nil, errors.New("error getting mirror RemoteAddress")
	}
	return &api.PushMirror{
		RepoName:       repo.Name,
		RemoteName:     pm.RemoteName,
		RemoteAddress:  remoteAddress,
		CreatedUnix:    pm.CreatedUnix.FormatLong(),
		LastUpdateUnix: pm.LastUpdateUnix.FormatLong(),
		LastError:      pm.LastError,
		Interval:       pm.Interval.String(),
	}, nil
}

func getMirrorRemoteAddress(repo *repo_model.Repository, remoteName string) (string, error) {
	url, err := git.GetRemoteURL(git.DefaultContext, repo.RepoPath(), remoteName)
	if err != nil {
		return "", err
	}
	// remove confidential information
	url.User = nil
	return url.String(), nil
}
