// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import (
	"gitea.dev/models/db"
	"gitea.dev/models/migrations/base"
	"gitea.dev/modules/timeutil"
)

// AddActionEnvironmentTables creates the action_environment table, adds environment_id
// to secret and action_variable, and adds environment_name to action_run_job.
func AddActionEnvironmentTables(x db.EngineMigration) error {
	type ActionEnvironment struct {
		ID                int64              `xorm:"pk autoincr"`
		RepoID            int64              `xorm:"UNIQUE(repo_name) NOT NULL"`
		Name              string             `xorm:"UNIQUE(repo_name) NOT NULL"`
		ProtectedBranches string             `xorm:"TEXT"`
		CreatedUnix       timeutil.TimeStamp `xorm:"created NOT NULL"`
		UpdatedUnix       timeutil.TimeStamp `xorm:"updated"`
	}

	if err := x.Sync(new(ActionEnvironment)); err != nil {
		return err
	}

	// Add the environment_id column to secret and action_variable first.
	// RecreateTable copies data by selecting the new column set from the
	// existing table, so environment_id must already exist before recreating.
	{
		type Secret struct {
			EnvironmentID int64 `xorm:"NOT NULL DEFAULT 0"`
		}
		if err := x.Sync(new(Secret)); err != nil {
			return err
		}
	}
	{
		type ActionVariable struct {
			EnvironmentID int64 `xorm:"NOT NULL DEFAULT 0"`
		}
		if err := x.Sync(new(ActionVariable)); err != nil {
			return err
		}
	}

	// Recreate the tables so environment_id becomes part of the unique constraint.
	type Secret struct {
		ID            int64              `xorm:"pk autoincr"`
		OwnerID       int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL"`
		RepoID        int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
		EnvironmentID int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
		Name          string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
		Data          string             `xorm:"LONGTEXT"`
		Description   string             `xorm:"TEXT"`
		CreatedUnix   timeutil.TimeStamp `xorm:"created NOT NULL"`
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := base.RecreateTable(sess, new(Secret)); err != nil {
		return err
	}

	type ActionVariable struct {
		ID            int64              `xorm:"pk autoincr"`
		OwnerID       int64              `xorm:"UNIQUE(owner_repo_name)"`
		RepoID        int64              `xorm:"INDEX UNIQUE(owner_repo_name)"`
		EnvironmentID int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
		Name          string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
		Data          string             `xorm:"LONGTEXT NOT NULL"`
		Description   string             `xorm:"TEXT"`
		CreatedUnix   timeutil.TimeStamp `xorm:"created NOT NULL"`
		UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
	}

	if err := base.RecreateTable(sess, new(ActionVariable)); err != nil {
		return err
	}
	if err := sess.Commit(); err != nil {
		return err
	}

	type ActionRunJob struct {
		EnvironmentName string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	}
	return x.Sync(new(ActionRunJob))
}
