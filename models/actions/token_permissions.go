// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/log"
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

	var actionsCfg *repo_model.ActionsConfig
	if actionsUnit, err := runRepo.GetUnit(ctx, unit.TypeActions); err == nil {
		actionsCfg = actionsUnit.ActionsConfig()
	} else {
		actionsCfg = &repo_model.ActionsConfig{}
	}

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

	if !actionsCfg.OverrideOrgConfig && runRepo.Owner.IsOrganization() {
		orgCfg, err := GetOrgActionsConfig(ctx, runRepo.OwnerID)
		if err == nil {
			effectivePerms = orgCfg.ClampPermissions(effectivePerms)
		} else {
			// At minimum log the error to avoid silent privilege escalation if store is unavailable
			log.Error("GetOrgActionsConfig failed for org %d: %v", runRepo.OwnerID, err)
		}
	}
	effectivePerms = actionsCfg.ClampPermissions(effectivePerms)

	isSameRepo := job.RepoID == targetRepo.ID
	maxReadOnly := job.Run.IsForkPullRequest || !isSameRepo
	if maxReadOnly {
		effectivePerms = effectivePerms.ClampPermissions(repo_model.GetReadOnlyPermissions())
	}

	return &effectivePerms, nil
}
