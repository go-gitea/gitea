// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	repo_model "code.gitea.io/gitea/internal/models/repo"
	api "code.gitea.io/gitea/internal/modules/structs"
)

// ToPushMirror convert from repo_model.PushMirror and remoteAddress to api.TopicResponse
func ToPushMirror(pm *repo_model.PushMirror) (*api.PushMirror, error) {
	repo := pm.GetRepository()
	return &api.PushMirror{
		RepoName:       repo.Name,
		RemoteName:     pm.RemoteName,
		RemoteAddress:  pm.RemoteAddress,
		CreatedUnix:    pm.CreatedUnix.FormatLong(),
		LastUpdateUnix: pm.LastUpdateUnix.FormatLong(),
		LastError:      pm.LastError,
		Interval:       pm.Interval.String(),
		SyncOnCommit:   pm.SyncOnCommit,
	}, nil
}
