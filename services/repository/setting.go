// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"slices"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/log"
	actions_service "gitea.dev/services/actions"
)

// UpdateRepositoryUnits updates a repository's units
func UpdateRepositoryUnits(ctx context.Context, repo *repo_model.Repository, units []repo_model.RepoUnit, deleteUnitTypes []unit.Type) (err error) {
	disableActions := slices.Contains(deleteUnitTypes, unit.TypeActions)

	err = db.WithTx(ctx, func(ctx context.Context) error {
		// Delete existing settings of units before adding again
		for _, u := range units {
			deleteUnitTypes = append(deleteUnitTypes, u.Type)
		}

		for _, u := range units {
			if u.Type == unit.TypeActions {
				if err := actions_model.DeleteScheduleTaskByRepo(ctx, repo.ID); err != nil {
					log.Error("DeleteScheduleTaskByRepo: %v", err)
				}
				if err := actions_service.DetectAndHandleSchedules(ctx, repo); err != nil {
					log.Error("DetectAndHandleSchedules: %v", err)
				}
				break
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

		return nil
	})
	if err != nil {
		return err
	}

	// only disabling stops in-progress schedule runs
	if disableActions {
		if err := actions_service.CleanRepoScheduleTasks(ctx, repo); err != nil {
			log.Error("CleanRepoScheduleTasks: %v", err)
		}
	}
	return nil
}
