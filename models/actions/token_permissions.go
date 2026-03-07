// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/util"
)

// ComputeJobTokenPermissions computes the effective permissions for a job token against the target repository.
// It uses the job's stored permissions (if any), then applies org/repo clamps and fork/cross-repo restrictions.
// Note: target repository access policy checks are enforced in GetActionsUserRepoPermission; this function only computes the job token's effective permission ceiling.
func ComputeJobTokenPermissions(ctx context.Context, job *ActionRunJob, targetRepo *repo_model.Repository) (repo_model.ActionsTokenPermissions, error) {
	if err := job.LoadRepo(ctx); err != nil {
		return repo_model.ActionsTokenPermissions{}, err
	}
	if err := job.LoadRun(ctx); err != nil {
		return repo_model.ActionsTokenPermissions{}, err
	}
	runRepo := job.Repo

	if err := runRepo.LoadOwner(ctx); err != nil {
		return repo_model.ActionsTokenPermissions{}, err
	}

	repoActionsCfg := runRepo.MustGetUnit(ctx, unit.TypeActions).ActionsConfig()
	ownerActionsCfg, err := GetUserActionsConfig(ctx, runRepo.OwnerID)
	if err != nil {
		return repo_model.ActionsTokenPermissions{}, err
	}

	var defaultPerms repo_model.ActionsTokenPermissions

	if job.TokenPermissions != "" {
		perms, err := repo_model.UnmarshalTokenPermissions(job.TokenPermissions)
		if err != nil {
			return repo_model.ActionsTokenPermissions{}, err
		}
		defaultPerms = perms
	} else {
		defaultPerms = util.Iif(repoActionsCfg.OverrideOwnerConfig, repoActionsCfg.GetDefaultTokenPermissions(), ownerActionsCfg.GetDefaultTokenPermissions())
	}

	effectivePerms := util.Iif(repoActionsCfg.OverrideOwnerConfig, repoActionsCfg.ClampPermissions(defaultPerms), ownerActionsCfg.ClampPermissions(defaultPerms))

	isSameRepo := job.RepoID == targetRepo.ID
	// Cross-repository access and fork pull requests are strictly read-only for security.
	// This ensures a "task repo" cannot gain write access to other repositories via CrossRepoAccess settings.
	maxReadOnly := job.Run.IsForkPullRequest || !isSameRepo
	if maxReadOnly {
		effectivePerms = effectivePerms.ClampPermissions(repo_model.GetRestrictedPermissions())
	}

	return effectivePerms, nil
}
