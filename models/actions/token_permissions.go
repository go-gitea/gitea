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
// Note: target repository access policy checks are enforced in GetActionsUserRepoPermission; this function only computes the job token's effective permission ceiling.
func ComputeJobTokenPermissions(ctx context.Context, job *ActionRunJob, targetRepo *repo_model.Repository) (ret repo_model.ActionsTokenPermissions, err error) {
	if err := job.LoadRepo(ctx); err != nil {
		return ret, err
	}
	if err := job.LoadRun(ctx); err != nil {
		return ret, err
	}
	runRepo := job.Repo

	if err := runRepo.LoadOwner(ctx); err != nil {
		return ret, err
	}

	repoActionsCfg := runRepo.MustGetUnit(ctx, unit.TypeActions).ActionsConfig()
	ownerActionsCfg, err := GetUserActionsConfig(ctx, runRepo.OwnerID)
	if err != nil {
		return ret, err
	}

	var jobDeclaredPerms repo_model.ActionsTokenPermissions
	if job.TokenPermissions != "" {
		perms, err := repo_model.UnmarshalTokenPermissions(job.TokenPermissions)
		if err != nil {
			return ret, err
		}
		jobDeclaredPerms = perms
	} else if repoActionsCfg.OverrideOwnerConfig {
		jobDeclaredPerms = repoActionsCfg.GetDefaultTokenPermissions()
	} else {
		jobDeclaredPerms = ownerActionsCfg.GetDefaultTokenPermissions()
	}

	var effectivePerms repo_model.ActionsTokenPermissions
	if repoActionsCfg.OverrideOwnerConfig {
		effectivePerms = repoActionsCfg.ClampPermissions(jobDeclaredPerms)
	} else {
		effectivePerms = ownerActionsCfg.ClampPermissions(jobDeclaredPerms)
	}

	// Cross-repository access and fork pull requests are strictly read-only for security.
	// This ensures a "task repo" cannot gain write access to other repositories via CrossRepoAccess settings.
	isSameRepo := job.RepoID == targetRepo.ID
	restrictCrossRepoAccess := job.Run.IsForkPullRequest || !isSameRepo
	if restrictCrossRepoAccess {
		effectivePerms = effectivePerms.ClampPermissions(repo_model.GetRestrictedPermissions())
	}

	return effectivePerms, nil
}
