// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func CreateActionTasksVersionTable(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	type ActionTasksVersion struct {
		ID          int64 `xorm:"pk autoincr"`
		OwnerID     int64 `xorm:"UNIQUE(owner_repo)"`
		RepoID      int64 `xorm:"INDEX UNIQUE(owner_repo)"`
		Version     int64
		CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
		UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
	}

	// cerate action_tasks_version table.
	if err := x.Sync(new(ActionTasksVersion)); err != nil {
		return err
	}

	// initialize data
	type ScopeItem struct {
		OwnerID int64
		RepoID  int64
	}
	scopes := []ScopeItem{}
	if err := sess.Distinct("owner_id", "repo_id").Table("action_runner").Where("deleted is null").Find(&scopes); err != nil {
		return err
	}

	if len(scopes) > 0 {
		versions := make([]ActionTasksVersion, 0, len(scopes))
		for _, scope := range scopes {
			versions = append(versions, ActionTasksVersion{
				OwnerID: scope.OwnerID,
				RepoID:  scope.RepoID,
				// Set the default value of version to 1, so that the first fetch request after the runner starts will definitely query the database.
				Version: 1,
			})
		}

		if _, err := sess.Insert(&versions); err != nil {
			return err
		}
	}

	return sess.Commit()
}
