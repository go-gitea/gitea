// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
)

// AddActionEnvironmentTables creates the action_environment, action_environment_secret,
// and action_environment_variable tables for the Environments feature.
func AddActionEnvironmentTables(x db.EngineMigration) error {
	type ActionEnvironment struct {
		ID                int64              `xorm:"pk autoincr"`
		RepoID            int64              `xorm:"UNIQUE(repo_name) NOT NULL"`
		Name              string             `xorm:"UNIQUE(repo_name) NOT NULL"`
		ProtectedBranches string             `xorm:"TEXT"`
		CreatedUnix       timeutil.TimeStamp `xorm:"created NOT NULL"`
		UpdatedUnix       timeutil.TimeStamp `xorm:"updated"`
	}

	type ActionEnvironmentSecret struct {
		ID            int64              `xorm:"pk autoincr"`
		RepoID        int64              `xorm:"UNIQUE(env_name) NOT NULL"`
		EnvironmentID int64              `xorm:"UNIQUE(env_name) NOT NULL"`
		Name          string             `xorm:"UNIQUE(env_name) NOT NULL"`
		Data          string             `xorm:"LONGTEXT"`
		Description   string             `xorm:"TEXT"`
		CreatedUnix   timeutil.TimeStamp `xorm:"created NOT NULL"`
	}

	type ActionEnvironmentVariable struct {
		ID            int64              `xorm:"pk autoincr"`
		RepoID        int64              `xorm:"UNIQUE(env_var_name) NOT NULL"`
		EnvironmentID int64              `xorm:"UNIQUE(env_var_name) NOT NULL"`
		Name          string             `xorm:"UNIQUE(env_var_name) NOT NULL"`
		Data          string             `xorm:"LONGTEXT NOT NULL"`
		Description   string             `xorm:"TEXT"`
		CreatedUnix   timeutil.TimeStamp `xorm:"created NOT NULL"`
		UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync(
		new(ActionEnvironment),
		new(ActionEnvironmentSecret),
		new(ActionEnvironmentVariable),
	)
}
