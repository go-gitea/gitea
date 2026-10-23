// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"maps"
	"slices"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/log"
)

// CommitActionsStatusMap maps CommitStatus.ID to the live ActionRunJob status
// for Gitea Actions rows.
type CommitActionsStatusMap map[int64]actions_model.Status

// maxJobIDsPerQuery keeps the IN clause below the bound-parameter limits of the supported databases.
const maxJobIDsPerQuery = 300

// IconStatus returns the action status name to route the icon through
// repo/icons/action_status, or "" when the row isn't from Gitea Actions.
func (m CommitActionsStatusMap) IconStatus(s *git_model.CommitStatus) string {
	if status, ok := m[s.ID]; ok {
		return status.String()
	}
	return ""
}

// GetCommitActionsStatusMap resolves the live ActionRunJob.Status for every
// CommitStatus row backed by Gitea Actions. Rows from other sources (external
// CIs, API) are left untouched and rendered from their stored State.
func GetCommitActionsStatusMap(ctx context.Context, statuses []*git_model.CommitStatus) CommitActionsStatusMap {
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
	for jobIDs := range slices.Chunk(slices.Collect(maps.Keys(statusByJobID)), maxJobIDsPerQuery) {
		if err := db.GetEngine(ctx).In("id", jobIDs).Cols("id", "status").Find(&jobs); err != nil {
			log.Error("db.Find: failed to find action run jobs: %v", err)
			return nil
		}
	}
	info := make(CommitActionsStatusMap, len(jobs))
	for jobID, status := range statusByJobID {
		if job, ok := jobs[jobID]; ok {
			info[status.ID] = job.Status
		}
	}
	return info
}

// commitActionsStatusCacheKey is the template data key of the preloaded CommitActionsStatusCache.
const commitActionsStatusCacheKey = "CommitActionsStatusCache"

// CommitActionsStatusCache holds the Actions statuses resolved in one batch for a set of commit statuses.
type CommitActionsStatusCache struct {
	covered map[int64]struct{}
	info    CommitActionsStatusMap
}

// PreloadCommitActionsStatuses resolves the statuses with one batch and stores the result in data,
// so that rendering many commits doesn't query the database once per commit.
func PreloadCommitActionsStatuses(ctx context.Context, data map[string]any, statuses []*git_model.CommitStatus) {
	if len(statuses) == 0 {
		return
	}
	cache := &CommitActionsStatusCache{covered: make(map[int64]struct{}, len(statuses))}
	for _, s := range statuses {
		if s != nil {
			cache.covered[s.ID] = struct{}{}
		}
	}
	cache.info = GetCommitActionsStatusMap(ctx, statuses)
	data[commitActionsStatusCacheKey] = cache
}

// GetCommitActionsStatusMapCached uses the preloaded cache in data if it covers all statuses,
// otherwise it falls back to GetCommitActionsStatusMap.
func GetCommitActionsStatusMapCached(ctx context.Context, data map[string]any, statuses []*git_model.CommitStatus) CommitActionsStatusMap {
	if cache, ok := data[commitActionsStatusCacheKey].(*CommitActionsStatusCache); ok {
		all := true
		for _, s := range statuses {
			if s == nil {
				continue
			}
			if _, ok := cache.covered[s.ID]; !ok {
				all = false
				break
			}
		}
		if all {
			return cache.info
		}
	}
	return GetCommitActionsStatusMap(ctx, statuses)
}
