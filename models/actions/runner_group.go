// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/container"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

type ActionRunnerGroup struct {
	ID                      int64  `xorm:"pk autoincr"`
	OwnerID                 int64  `xorm:"UNIQUE(owner_name) NOT NULL DEFAULT 0"`
	Name                    string `xorm:"VARCHAR(255) NOT NULL"`
	LowerName               string `xorm:"VARCHAR(255) UNIQUE(owner_name) NOT NULL"`
	IncludesAllRepositories bool   `xorm:"NOT NULL DEFAULT false"`
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
	err := db.GetEngine(ctx).Cols("id", "name", "group_id").Where("owner_id = ? AND repo_id = 0", ownerID).Asc("name").Find(&runners)
	return runners, err
}

func UpdateRunnerGroup(ctx context.Context, group *ActionRunnerGroup, includesAllRepositories bool, repoIDs, runnerIDs []int64) error {
	if includesAllRepositories {
		repoIDs = nil
	}
	repoIDs, runnerIDs = container.SetOf(repoIDs...).Values(), container.SetOf(runnerIDs...).Values()
	return db.WithTx(ctx, func(ctx context.Context) error {
		group.IncludesAllRepositories = includesAllRepositories
		if err := lockRunnerGroup(ctx, group); err != nil {
			return err
		}
		if err := setRunnerAccess(ctx, group, repoIDs); err != nil {
			return err
		}
		if err := setRunnerGroupMembers(ctx, group, runnerIDs); err != nil {
			return err
		}
		return IncreaseTaskVersion(ctx, group.OwnerID, 0)
	})
}

// lockRunnerGroup writes the row to lock it, FOR UPDATE isn't portable across databases
func lockRunnerGroup(ctx context.Context, group *ActionRunnerGroup) error {
	if _, err := db.GetEngine(ctx).ID(group.ID).Cols("includes_all_repositories").Update(group); err != nil {
		return err
	}
	exists, err := db.ExistByID[ActionRunnerGroup](ctx, group.ID)
	if err != nil {
		return err
	}
	if !exists {
		return util.NewNotExistErrorf("the runner group no longer exists")
	}
	return nil
}

func setRunnerGroupMembers(ctx context.Context, group *ActionRunnerGroup, runnerIDs []int64) error {
	if len(runnerIDs) > 0 {
		inScopeCount, err := db.GetEngine(ctx).Where(builder.Eq{"owner_id": group.OwnerID, "repo_id": 0}.And(builder.In("id", runnerIDs))).Count(new(ActionRunner))
		if err != nil {
			return err
		}
		if inScopeCount != int64(len(runnerIDs)) {
			return util.NewPermissionDeniedErrorf("runner is outside the group's scope")
		}
	}
	released := builder.Eq{"group_id": group.ID}.And(builder.NotIn("id", runnerIDs))
	if _, err := db.GetEngine(ctx).Where(released).Cols("group_id").Update(&ActionRunner{GroupID: 0}); err != nil {
		return err
	}
	if len(runnerIDs) > 0 {
		if _, err := db.GetEngine(ctx).In("id", runnerIDs).Cols("group_id").Update(&ActionRunner{GroupID: group.ID}); err != nil {
			return err
		}
	}
	return nil
}

func setRunnerAccess(ctx context.Context, group *ActionRunnerGroup, repoIDs []int64) error {
	if len(repoIDs) > 0 {
		inScope := builder.In("id", repoIDs)
		if group.OwnerID != 0 {
			inScope = inScope.And(builder.Eq{"owner_id": group.OwnerID})
		}
		inScopeCount, err := db.GetEngine(ctx).Where(inScope).Count(new(repo_model.Repository))
		if err != nil {
			return err
		}
		if inScopeCount != int64(len(repoIDs)) {
			return util.NewPermissionDeniedErrorf("repository is outside the group's scope")
		}
	}
	if err := db.DeleteBeans(ctx, &ActionRunnerAccess{GroupID: group.ID}); err != nil {
		return err
	}
	rows := make([]*ActionRunnerAccess, 0, len(repoIDs))
	for _, repoID := range repoIDs {
		rows = append(rows, &ActionRunnerAccess{GroupID: group.ID, RepoID: repoID})
	}
	for chunk := range slices.Chunk(rows, db.MaxBatchInsertSize(new(ActionRunnerAccess))) {
		if err := db.Insert(ctx, chunk); err != nil {
			return err
		}
	}
	return nil
}

func PruneRunnerAccessOutsideOwner(ctx context.Context, repoID int64) error {
	stale := builder.Select("id").From("action_runner_group").Where(builder.Neq{"owner_id": 0})
	_, err := db.GetEngine(ctx).Where(builder.Eq{"repo_id": repoID}).And(builder.In("group_id", stale)).
		Delete(new(ActionRunnerAccess))
	return err
}

func CountRunnerGroupUsage(ctx context.Context, groupIDs []int64) (runners, repos map[int64]int64, err error) {
	countBy := func(table string, cond builder.Cond) (map[int64]int64, error) {
		var rows []struct {
			GroupID int64
			Count   int64
		}
		if err := db.GetEngine(ctx).Table(table).Select("group_id, COUNT(*) AS count").
			Where(builder.In("group_id", groupIDs).And(cond)).GroupBy("group_id").Find(&rows); err != nil {
			return nil, err
		}
		counts := make(map[int64]int64, len(rows))
		for _, row := range rows {
			counts[row.GroupID] = row.Count
		}
		return counts, nil
	}
	if len(groupIDs) == 0 {
		return nil, nil, nil
	}
	if runners, err = countBy("action_runner", builder.IsNull{"deleted"}); err != nil {
		return nil, nil, err
	}
	repos, err = countBy("action_runner_access", nil)
	return runners, repos, err
}

func CreateRunnerGroup(ctx context.Context, ownerID int64, name string) (*ActionRunnerGroup, error) {
	if name == "" || utf8.RuneCountInString(name) > 200 || strings.Contains(name, "/") { // 200 leaves room for "organization/" in runs_on_group
		return nil, util.NewInvalidArgumentErrorf("invalid runner group name %q", name)
	}
	group := &ActionRunnerGroup{OwnerID: ownerID, Name: name, LowerName: strings.ToLower(name)}
	cond := builder.Eq{"lower_name": group.LowerName}
	if ownerID != 0 {
		cond["owner_id"] = []int64{ownerID, 0}
	}
	exists, err := db.GetEngine(ctx).Where(cond).Exist(new(ActionRunnerGroup)) // global and owner names never overlap, an unprefixed runs-on.group would match both
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, util.NewAlreadyExistErrorf("runner group %q already exists", name)
	}
	return group, db.Insert(ctx, group)
}

func DeleteRunnerGroup(ctx context.Context, group *ActionRunnerGroup) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := lockRunnerGroup(ctx, group); err != nil {
			return err
		}
		hasRunners, err := db.GetEngine(ctx).Where(builder.Eq{"group_id": group.ID}).Exist(new(ActionRunner))
		if err != nil {
			return err
		}
		if hasRunners {
			return util.NewInvalidArgumentErrorf("the runner group still has runners")
		}
		return db.DeleteBeans(ctx, &ActionRunnerGroup{ID: group.ID}, &ActionRunnerAccess{GroupID: group.ID})
	})
}

func DeleteRunnerGroupsByOwner(ctx context.Context, ownerID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		groupIDs, err := db.FindIDs(ctx, "action_runner_group", "id", builder.Eq{"owner_id": ownerID})
		if err != nil || len(groupIDs) == 0 {
			return err
		}
		if err := db.DeleteByIDs[ActionRunnerGroup](ctx, groupIDs...); err != nil {
			return err
		}
		_, err = db.GetEngine(ctx).In("group_id", groupIDs).Delete(new(ActionRunnerAccess))
		return err
	})
}

func FindRunnerGroupRepos(ctx context.Context, group *ActionRunnerGroup) (repo_model.RepositoryList, error) {
	cond := builder.Eq{"action_runner_access.group_id": group.ID}
	if group.OwnerID != 0 {
		cond["repository.owner_id"] = group.OwnerID // skips grants a concurrent transfer left behind
	}
	var repos repo_model.RepositoryList
	err := db.GetEngine(ctx).Cols("repository.id", "repository.owner_name", "repository.name").
		Join("INNER", "action_runner_access", "action_runner_access.repo_id = repository.id").
		Where(cond).Asc("repository.lower_name").Find(&repos)
	return repos, err
}
