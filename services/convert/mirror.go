// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	repo_model "gitea.dev/models/repo"
	api "gitea.dev/modules/structs"
)

// ToPushMirror convert from repo_model.PushMirror and remoteAddress to api.TopicResponse
func ToPushMirror(ctx context.Context, pm *repo_model.PushMirror) (*api.PushMirror, error) {
	repo := pm.GetRepository(ctx)
	branchFilters := pm.Config.BranchFilters
	if branchFilters == nil {
		branchFilters = []string{}
	}
	return &api.PushMirror{
		RepoName:       repo.Name,
		RemoteName:     pm.RemoteName,
		RemoteAddress:  pm.RemoteAddress,
		CreatedUnix:    pm.CreatedUnix.AsTime(),
		LastUpdateUnix: pm.LastUpdateUnix.AsTimePtr(),
		LastError:      pm.LastError,
		Interval:       pm.Interval.String(),
		SyncOnCommit:   pm.SyncOnCommit,
		Config: api.PushMirrorConfig{
			KeepRemoteBranches: pm.Config.KeepRemoteBranches,
			NoPushTags:         pm.Config.NoPushTags,
			KeepRemoteTags:     pm.Config.KeepRemoteTags,
			BranchFilters:      branchFilters,
		},
	}, nil
}

// ToPushMirrorHistoryList converts push mirror histories to API objects
func ToPushMirrorHistoryList(list []*repo_model.PushMirrorHistory) []*api.PushMirrorHistory {
	res := make([]*api.PushMirrorHistory, 0, len(list))
	for _, h := range list {
		failed := make([]api.PushMirrorRefError, 0, len(h.Result.Failed))
		for _, f := range h.Result.Failed {
			failed = append(failed, api.PushMirrorRefError{Ref: f.Ref, Reason: f.Reason})
		}
		res = append(res, &api.PushMirrorHistory{
			Status:      string(h.Status),
			Created:     h.CreatedUnix.AsTime(),
			DurationMs:  h.DurationMs,
			Pushed:      h.Result.Pushed,
			Deleted:     h.Result.Deleted,
			UpToDate:    h.Result.UpToDate,
			FailedTotal: h.Result.FailedTotal,
			Failed:      failed,
			Error:       h.Result.Error,
		})
	}
	return res
}
