// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package commitstatusinfo

import (
	"context"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
)

// CommitStatusActionInfo maps CommitStatus.ID to the live ActionRunJob status
// for Gitea Actions rows.
type CommitStatusActionInfo map[int64]actions_model.Status

// IconStatus returns the action status name to route the icon through
// repo/icons/action_status, or "" when the row isn't from Gitea Actions.
func (m CommitStatusActionInfo) IconStatus(s *git_model.CommitStatus) string {
	if status, ok := m[s.ID]; ok {
		return status.String()
	}
	return ""
}

// GetCommitStatusActionInfo resolves the live ActionRunJob.Status for every
// CommitStatus row backed by Gitea Actions. Rows from other sources (external
// CIs, API) are left untouched and rendered from their stored State.
func GetCommitStatusActionInfo(ctx context.Context, statuses []*git_model.CommitStatus) CommitStatusActionInfo {
	if len(statuses) == 0 {
		return nil
	}
	statusByJobID := make(map[int64]*git_model.CommitStatus)
	repoCache := make(map[int64]*repo_model.Repository)
	for _, status := range statuses {
		if status == nil || status.TargetURL == "" {
			continue
		}
		if status.Repo == nil {
			status.Repo = repoCache[status.RepoID]
		}
		_, jobID, ok := status.ParseGiteaActionsTargetURL(ctx)
		repoCache[status.RepoID] = status.Repo
		if ok {
			statusByJobID[jobID] = status
		}
	}
	if len(statusByJobID) == 0 {
		return nil
	}
	jobIDs := make([]int64, 0, len(statusByJobID))
	for id := range statusByJobID {
		jobIDs = append(jobIDs, id)
	}
	jobs := make(map[int64]*actions_model.ActionRunJob, len(jobIDs))
	if err := db.GetEngine(ctx).In("id", jobIDs).Cols("id", "status").Find(&jobs); err != nil {
		log.Error("GetCommitStatusActionInfo: find action run jobs: %v", err)
		return nil
	}
	info := make(CommitStatusActionInfo, len(jobs))
	for jobID, status := range statusByJobID {
		if job, ok := jobs[jobID]; ok && !job.Status.IsUnknown() {
			info[status.ID] = job.Status
		}
	}
	return info
}
