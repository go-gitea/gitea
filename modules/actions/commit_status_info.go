// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"maps"
	"slices"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
)

// CommitStatusInfo maps CommitStatus.ID to the live ActionRunJob status
// for Gitea Actions rows.
type CommitStatusInfo map[int64]actions_model.Status

// IconStatus returns the action status name to route the icon through
// repo/icons/action_status, or "" when the row isn't from Gitea Actions.
func (m CommitStatusInfo) IconStatus(s *git_model.CommitStatus) string {
	if status, ok := m[s.ID]; ok {
		return status.String()
	}
	return ""
}

// GetCommitStatusInfo resolves the live ActionRunJob.Status for every
// CommitStatus row backed by Gitea Actions. Rows from other sources (external
// CIs, API) are left untouched and rendered from their stored State.
//
// Mutates status.Repo for inputs whose Repo is nil — ParseGiteaActionsTargetURL needs it loaded.
func GetCommitStatusInfo(ctx context.Context, statuses []*git_model.CommitStatus) CommitStatusInfo {
	if len(statuses) == 0 {
		return nil
	}
	statusByJobID := make(map[int64]*git_model.CommitStatus)
	repoByID := make(map[int64]*repo_model.Repository)
	for _, status := range statuses {
		if status == nil || status.TargetURL == "" {
			continue
		}
		if status.Repo == nil {
			status.Repo = repoByID[status.RepoID]
		}
		// ParseGiteaActionsTargetURL lazy-loads status.Repo on miss; cache the
		// outcome so later entries with the same RepoID skip that load.
		_, jobID, ok := status.ParseGiteaActionsTargetURL(ctx)
		repoByID[status.RepoID] = status.Repo
		if ok {
			statusByJobID[jobID] = status
		}
	}
	if len(statusByJobID) == 0 {
		return nil
	}
	jobs := make(map[int64]*actions_model.ActionRunJob, len(statusByJobID))
	if err := db.GetEngine(ctx).In("id", slices.Collect(maps.Keys(statusByJobID))).Cols("id", "status").Find(&jobs); err != nil {
		log.Error("GetCommitStatusInfo: find action run jobs: %v", err)
		return nil
	}
	info := make(CommitStatusInfo, len(jobs))
	for jobID, status := range statusByJobID {
		if job, ok := jobs[jobID]; ok && !job.Status.IsUnknown() {
			info[status.ID] = job.Status
		}
	}
	return info
}
