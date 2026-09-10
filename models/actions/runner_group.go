// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/container"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

type ActionRunnerGroup struct {
	ID      int64  `xorm:"pk autoincr"`
	OwnerID int64  `xorm:"UNIQUE(owner_name) NOT NULL DEFAULT 0"`
	Name    string `xorm:"VARCHAR(255) UNIQUE(owner_name) NOT NULL"`
}

type ActionRunnerAccess struct {
	ID      int64 `xorm:"pk autoincr"`
	GroupID int64 `xorm:"UNIQUE(group_repo) NOT NULL"`
	RepoID  int64 `xorm:"INDEX UNIQUE(group_repo) NOT NULL"`
}

func init() {
	db.RegisterModel(new(ActionRunnerGroup))
	db.RegisterModel(new(ActionRunnerAccess))
}

func FindRunnerGroups(ctx context.Context, ownerID int64) ([]*ActionRunnerGroup, error) {
	var groups []*ActionRunnerGroup
	err := db.GetEngine(ctx).Where("owner_id = ?", ownerID).Asc("name").Find(&groups)
	return groups, err
}

func FindRunnerGroupCandidates(ctx context.Context, ownerID int64) ([]*ActionRunner, error) {
	var runners []*ActionRunner
	err := db.GetEngine(ctx).Where("owner_id = ? AND repo_id = 0", ownerID).Asc("name").Find(&runners)
	return runners, err
}

func SetRunnerGroupMembers(ctx context.Context, group *ActionRunnerGroup, runnerIDs []int64) error {
	candidates, err := FindRunnerGroupCandidates(ctx, group.OwnerID)
	if err != nil {
		return err
	}
	inScope := container.SetOf(container.FilterSlice(candidates, func(r *ActionRunner) (int64, bool) {
		return r.ID, true
	})...)
	if !inScope.Contains(runnerIDs...) {
		return util.NewPermissionDeniedErrorf("runner is outside the group's scope")
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		var released builder.Cond = builder.Eq{"group_id": group.ID}
		if len(runnerIDs) > 0 {
			released = released.And(builder.NotIn("id", runnerIDs))
		}
		if _, err := db.GetEngine(ctx).Where(released).Cols("group_id").Update(&ActionRunner{GroupID: 0}); err != nil {
			return err
		}
		if len(runnerIDs) > 0 {
			if _, err := db.GetEngine(ctx).In("id", runnerIDs).Cols("group_id").Update(&ActionRunner{GroupID: group.ID}); err != nil {
				return err
			}
		}
		return IncreaseTaskVersion(ctx, group.OwnerID, 0)
	})
}

func PruneRunnerAccessOutsideOwner(ctx context.Context, repoID, ownerID int64) error {
	stale := builder.Select("id").From("action_runner_group").
		Where(builder.Neq{"owner_id": 0}).And(builder.Neq{"owner_id": ownerID})
	_, err := db.GetEngine(ctx).Where(builder.Eq{"repo_id": repoID}).And(builder.In("group_id", stale)).
		Delete(new(ActionRunnerAccess))
	return err
}

func RunnerGroupsAllowingRepo(ctx context.Context, repoID int64) (container.Set[int64], error) {
	groupIDs, err := db.FindIDs(ctx, "action_runner_access", "group_id", builder.Eq{"repo_id": repoID})
	return container.SetOf(groupIDs...), err
}

func CreateRunnerGroup(ctx context.Context, ownerID int64, name string) (*ActionRunnerGroup, error) {
	group := &ActionRunnerGroup{OwnerID: ownerID, Name: name}
	return group, db.WithTx(ctx, func(ctx context.Context) error {
		exists, err := db.GetEngine(ctx).Where("owner_id = ? AND name = ?", ownerID, name).Exist(new(ActionRunnerGroup))
		if err != nil {
			return err
		}
		if exists {
			return util.NewAlreadyExistErrorf("runner group %q already exists", name)
		}
		return db.Insert(ctx, group)
	})
}

func DeleteRunnerGroup(ctx context.Context, group *ActionRunnerGroup) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		// conditional so a concurrent join can't leave runners pointing at a deleted group, which would widen what they may run
		hasRunners := builder.Exists(builder.Select("1").From("action_runner").
			Where(builder.Eq{"group_id": group.ID}).And(builder.IsNull{"deleted"}))
		n, err := db.GetEngine(ctx).Where(builder.Eq{"id": group.ID}).And(builder.Not{hasRunners}).Delete(new(ActionRunnerGroup))
		if err != nil {
			return err
		}
		if n == 0 {
			return util.NewInvalidArgumentErrorf("the runner group still has runners")
		}
		return db.DeleteBeans(ctx, &ActionRunnerAccess{GroupID: group.ID})
	})
}

func DeleteRunnerGroupsByOwner(ctx context.Context, ownerID int64) error {
	groupIDs := builder.Select("id").From("action_runner_group").Where(builder.Eq{"owner_id": ownerID})
	if _, err := db.GetEngine(ctx).Where(builder.In("group_id", groupIDs)).Delete(new(ActionRunnerAccess)); err != nil {
		return err
	}
	_, err := db.GetEngine(ctx).Where("owner_id = ?", ownerID).Delete(new(ActionRunnerGroup))
	return err
}

func FindRunnerGroupRepos(ctx context.Context, groupID int64) ([]*repo_model.Repository, error) {
	var repos []*repo_model.Repository
	err := db.GetEngine(ctx).
		Join("INNER", "action_runner_access", "action_runner_access.repo_id = repository.id").
		Where("action_runner_access.group_id = ?", groupID).Asc("repository.lower_name").Find(&repos)
	return repos, err
}

func SetRunnerAccess(ctx context.Context, group *ActionRunnerGroup, repoIDs []int64) error {
	repos, err := repo_model.GetRepositoriesMapByIDs(ctx, repoIDs)
	if err != nil {
		return err
	}
	rows := make([]*ActionRunnerAccess, 0, len(repoIDs))
	for _, repoID := range repoIDs {
		repo, ok := repos[repoID]
		if !ok || (group.OwnerID != 0 && repo.OwnerID != group.OwnerID) {
			return util.NewPermissionDeniedErrorf("repository is outside the group's scope")
		}
		rows = append(rows, &ActionRunnerAccess{GroupID: group.ID, RepoID: repoID})
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := db.DeleteBeans(ctx, &ActionRunnerAccess{GroupID: group.ID}); err != nil {
			return err
		}
		if len(rows) > 0 {
			if err := db.Insert(ctx, rows); err != nil {
				return err
			}
		}
		return IncreaseTaskVersion(ctx, group.OwnerID, 0) // wake members for work that just became eligible
	})
}
