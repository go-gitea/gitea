// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package access_test

import (
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/require"
)

func TestActionsPackagePermissions(t *testing.T) {
	for _, scenario := range []string{"write", "read", "none", "owner-clamp", "repo-clamp", "fork", "job-fork", "finished", "superseded", "cross-owner", "archived", "no-actions", "no-packages"} {
		t.Run(scenario, func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())
			ctx := t.Context()
			task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 47})
			job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: task.JobID})
			repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: task.RepoID})
			require.NoError(t, repo.LoadOwner(ctx))
			if !repo.UnitEnabled(ctx, unit.TypePackages) {
				require.NoError(t, db.Insert(ctx, &repo_model.RepoUnit{RepoID: repo.ID, Type: unit.TypePackages, Config: &repo_model.UnitConfig{}}))
			}
			permissions := repo_model.MakeActionsTokenPermissions(perm.AccessModeNone)
			permissions.UnitAccessModes[unit.TypePackages] = perm.AccessModeWrite
			expected := perm.AccessModeNone
			ownerID := repo.OwnerID
			switch scenario {
			case "write":
				expected = perm.AccessModeWrite
			case "read":
				permissions.UnitAccessModes[unit.TypePackages] = perm.AccessModeRead
				expected = perm.AccessModeRead
			case "none":
				permissions.UnitAccessModes[unit.TypePackages] = perm.AccessModeNone
			case "owner-clamp", "repo-clamp":
				ceiling := repo_model.MakeActionsTokenPermissions(perm.AccessModeNone)
				ceiling.UnitAccessModes[unit.TypePackages] = perm.AccessModeRead
				expected = perm.AccessModeRead
				if scenario == "owner-clamp" {
					require.NoError(t, actions_model.SetOwnerActionsConfig(ctx, ownerID, actions_model.OwnerActionsConfig{MaxTokenPermissions: &ceiling}))
				} else {
					u, err := repo.GetUnit(ctx, unit.TypeActions)
					require.NoError(t, err)
					u.Config = &repo_model.ActionsConfig{OverrideOwnerConfig: true, MaxTokenPermissions: &ceiling}
					_, err = db.GetEngine(ctx).ID(u.ID).Cols("config").Update(u)
					require.NoError(t, err)
				}
			case "fork":
				task.IsForkPullRequest = true
				expected = perm.AccessModeRead
			case "job-fork":
				job.IsForkPullRequest = true
				expected = perm.AccessModeRead
			case "finished":
				task.Status = actions_model.StatusSuccess
			case "superseded":
				job.TaskID++
			case "cross-owner":
				ownerID++
			case "no-actions", "no-packages":
				unitType := unit.TypeActions
				if scenario == "no-packages" {
					unitType = unit.TypePackages
				}
				_, err := db.GetEngine(ctx).Where("repo_id = ? AND type = ?", repo.ID, unitType).Delete(&repo_model.RepoUnit{})
				require.NoError(t, err)
			case "archived":
				repo.IsArchived = true
				_, err := db.GetEngine(ctx).ID(repo.ID).Cols("is_archived").Update(repo)
				require.NoError(t, err)
			}
			job.TokenPermissions = &permissions
			_, err := db.GetEngine(ctx).ID(job.ID).Cols("token_permissions", "task_id", "is_fork_pull_request").Update(job)
			require.NoError(t, err)
			_, err = db.GetEngine(ctx).ID(task.ID).Cols("status", "is_fork_pull_request").Update(task)
			require.NoError(t, err)
			mode, _, err := actions_model.GetActionsPackageAccessMode(ctx, task.ID, ownerID)
			require.NoError(t, err)
			require.Equal(t, expected, mode)
		})
	}
}
