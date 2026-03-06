// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
)

// ComputeJobTokenPermissions computes the effective permissions for a job token against the target repository.
// It uses the job's stored permissions (if any), then applies org/repo clamps and fork/cross-repo restrictions.
func ComputeJobTokenPermissions(ctx context.Context, job *ActionRunJob, targetRepo *repo_model.Repository) (*repo_model.ActionsTokenPermissions, error) {
	if err := job.LoadRepo(ctx); err != nil {
		return nil, err
	}
	if err := job.LoadRun(ctx); err != nil {
		return nil, err
	}
	runRepo := job.Repo

	if err := runRepo.LoadOwner(ctx); err != nil {
		return nil, err
	}

	actionsCfg := runRepo.MustGetUnit(ctx, unit.TypeActions).ActionsConfig()

	var effectivePerms repo_model.ActionsTokenPermissions
	if job.TokenPermissions != "" {
		perms, err := repo_model.UnmarshalTokenPermissions(job.TokenPermissions)
		if err != nil {
			return nil, err
		}
		effectivePerms = perms
	} else {
		effectivePerms = actionsCfg.GetDefaultTokenPermissions()
	}

	if !actionsCfg.OverrideOwnerConfig {
		ownerCfg, err := GetUserActionsConfig(ctx, runRepo.OwnerID)
		if err != nil {
			return nil, err
		}
		effectivePerms = ownerCfg.ClampPermissions(effectivePerms)
	}
	effectivePerms = actionsCfg.ClampPermissions(effectivePerms)

	isSameRepo := job.RepoID == targetRepo.ID
	// Cross-repository access and fork pull requests are strictly read-only for security.
	// This ensures a "task repo" cannot gain write access to other repositories via CrossRepoAccess settings.
	maxReadOnly := job.Run.IsForkPullRequest || !isSameRepo
	if maxReadOnly {
		effectivePerms = effectivePerms.ClampPermissions(repo_model.GetRestrictedPermissions())
	}

	return &effectivePerms, nil
}
