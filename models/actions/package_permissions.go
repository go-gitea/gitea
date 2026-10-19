// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
)

// GetActionsPackageAccessMode resolves a running task's package permissions in its repository owner's namespace.
func GetActionsPackageAccessMode(ctx context.Context, taskID, ownerID int64) (perm.AccessMode, int64, error) {
	task, err := GetTaskByID(ctx, taskID)
	if err != nil {
		return perm.AccessModeNone, 0, err
	}
	if task.Status != StatusRunning && task.Status != StatusCancelling {
		return perm.AccessModeNone, 0, nil
	}
	if err := task.LoadJob(ctx); err != nil {
		return perm.AccessModeNone, 0, err
	}
	if task.Job.TaskID != task.ID {
		return perm.AccessModeNone, 0, nil
	}
	if err := task.Job.LoadRepo(ctx); err != nil {
		return perm.AccessModeNone, 0, err
	}
	repo := task.Job.Repo
	if repo.OwnerID != ownerID || repo.IsArchived {
		return perm.AccessModeNone, 0, nil
	}
	if _, err := repo.GetUnit(ctx, unit.TypeActions); err != nil {
		if repo_model.IsErrUnitTypeNotExist(err) {
			return perm.AccessModeNone, 0, nil
		}
		return perm.AccessModeNone, 0, err
	}
	if _, err := repo.GetUnit(ctx, unit.TypePackages); err != nil {
		if repo_model.IsErrUnitTypeNotExist(err) {
			return perm.AccessModeNone, 0, nil
		}
		return perm.AccessModeNone, 0, err
	}
	permissions, err := ComputeTaskTokenPermissions(ctx, task, repo)
	if err != nil {
		return perm.AccessModeNone, 0, err
	}
	mode := min(permissions.UnitAccessModes[unit.TypePackages], perm.AccessModeWrite)
	if task.IsForkPullRequest || task.Job.IsForkPullRequest {
		mode = min(mode, perm.AccessModeRead)
	}
	return mode, repo.ID, nil
}
