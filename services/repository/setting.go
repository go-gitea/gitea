// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"slices"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	webhook_module "code.gitea.io/gitea/modules/webhook"
)

// UpdateRepositoryUnits updates a repository's units
func UpdateRepositoryUnits(ctx context.Context, repo *repo_model.Repository, units []repo_model.RepoUnit, deleteUnitTypes []unit.Type) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	// Delete existing settings of units before adding again
	for _, u := range units {
		deleteUnitTypes = append(deleteUnitTypes, u.Type)
	}

	if slices.Contains(deleteUnitTypes, unit.TypeActions) {
		if err := actions_model.CancelRunningJobs(
			ctx,
			repo.ID,
			repo.DefaultBranch,
			"",
			webhook_module.HookEventSchedule,
		); err != nil {
			return fmt.Errorf("CancelRunningJobs: %w", err)
		}
	}

	if _, err = db.GetEngine(ctx).Where("repo_id = ?", repo.ID).In("type", deleteUnitTypes).Delete(new(repo_model.RepoUnit)); err != nil {
		return err
	}

	if len(units) > 0 {
		if err = db.Insert(ctx, units); err != nil {
			return err
		}
	}

	return committer.Commit()
}
