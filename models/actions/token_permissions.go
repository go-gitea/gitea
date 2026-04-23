// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
)

// ComputeTaskTokenPermissions computes the effective permissions for a job token against the target repository.
// It uses the job's stored permissions (if any), then applies org/repo clamps and fork/cross-repo restrictions.
// Note: target repository access policy checks are enforced in GetActionsUserRepoPermission; this function only computes the job token's effective permission ceiling.
func ComputeTaskTokenPermissions(ctx context.Context, task *ActionTask, targetRepo *repo_model.Repository) (ret repo_model.ActionsTokenPermissions, err error) {
	if err := task.LoadJob(ctx); err != nil {
		return ret, err
	}
	if err := task.Job.LoadRepo(ctx); err != nil {
		return ret, err
	}
	runRepo := task.Job.Repo

	if err := runRepo.LoadOwner(ctx); err != nil {
		return ret, err
	}

	repoActionsCfg := runRepo.MustGetUnit(ctx, unit.TypeActions).ActionsConfig()
	ownerActionsCfg, err := GetOwnerActionsConfig(ctx, runRepo.OwnerID)
	if err != nil {
		return ret, err
	}

	var jobDeclaredPerms repo_model.ActionsTokenPermissions
	if task.Job.TokenPermissions != nil {
		jobDeclaredPerms = *task.Job.TokenPermissions
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
	isSameRepo := task.Job.RepoID == targetRepo.ID
	restrictCrossRepoAccess := task.IsForkPullRequest || !isSameRepo
	if restrictCrossRepoAccess {
		effectivePerms = repo_model.ClampActionsTokenPermissions(effectivePerms, repo_model.MakeRestrictedPermissions())
	}

	return effectivePerms, nil
}
