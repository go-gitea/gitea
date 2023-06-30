// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

type ActionTasksVersion struct {
	ID          int64 `xorm:"pk autoincr"`
	OwnerID     int64 `xorm:"UNIQUE(owner_repo)"`
	RepoID      int64 `xorm:"INDEX UNIQUE(owner_repo)"`
	Version     int64
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ActionTasksVersion))
}

func GetTasksVersionByScope(ctx context.Context, ownerID, repoID int64) (*ActionTasksVersion, error) {
	var tasksVersion ActionTasksVersion
	has, err := db.GetEngine(ctx).Where("owner_id = ? AND repo_id = ?", ownerID, repoID).Get(&tasksVersion)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("tasks version with owner id %d repo id %d: %w", ownerID, repoID, util.ErrNotExist)
	}
	return &tasksVersion, err
}

func InsertTasksVersion(ctx context.Context, ownerID, repoID int64) (*ActionTasksVersion, error) {
	tasksVersion := &ActionTasksVersion{
		OwnerID: ownerID,
		RepoID:  repoID,
		// Set the default value of version to 1, so that the first fetch request after the runner starts will definitely query the database.
		Version: 1,
	}
	return tasksVersion, db.Insert(ctx, tasksVersion)
}

func increaseTasksVersionByScope(ctx context.Context, ownerID, repoID int64) (bool, error) {
	result, err := db.GetEngine(ctx).Exec("UPDATE action_tasks_version SET version = version + 1 WHERE owner_id = ? AND repo_id = ?", ownerID, repoID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected != 0, err
}

func IncreaseTaskVersion(ctx context.Context, ownerID, repoID int64) error {
	// increase tasks version
	// 1. increase global
	if _, err := increaseTasksVersionByScope(ctx, 0, 0); err != nil {
		log.Error("IncreaseTasksVersionByScope(Global): %v", err)
		return err
	}
	// 2. increase owner
	if _, err := increaseTasksVersionByScope(ctx, ownerID, 0); err != nil {
		log.Error("IncreaseTasksVersionByScope(Owner): %v", err)
		return err
	}
	// 3. increase repo
	if _, err := increaseTasksVersionByScope(ctx, 0, repoID); err != nil {
		log.Error("IncreaseTasksVersionByScope(Repo): %v", err)
		return err
	}
	return nil
}
