// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"
)

func AddActionEnvironmentSchema(_ context.Context, x base.EngineMigration) error {
	type ActionEnvironment struct {
		ID                    int64              `xorm:"pk autoincr"`
		RepoID                int64              `xorm:"UNIQUE(repo_lower_name) NOT NULL"`
		Name                  string             `xorm:"NOT NULL"`
		LowerName             string             `xorm:"UNIQUE(repo_lower_name) NOT NULL"`
		AllowedBranchPatterns string             `xorm:"TEXT"`
		CreatedUnix           timeutil.TimeStamp `xorm:"created NOT NULL"`
		UpdatedUnix           timeutil.TimeStamp `xorm:"updated"`
	}

	// declared in full, so that Sync replaces the unique name index with the one covering environment_id
	type Secret struct {
		ID            int64              `xorm:"pk autoincr"`
		OwnerID       int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL"`
		RepoID        int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
		EnvironmentID int64              `xorm:"UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
		Name          string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
		Data          string             `xorm:"LONGTEXT"`
		Description   string             `xorm:"TEXT"`
		CreatedUnix   timeutil.TimeStamp `xorm:"created NOT NULL"`
	}
	type ActionVariable struct {
		ID            int64              `xorm:"pk autoincr"`
		OwnerID       int64              `xorm:"UNIQUE(owner_repo_name)"`
		RepoID        int64              `xorm:"INDEX UNIQUE(owner_repo_name)"`
		EnvironmentID int64              `xorm:"UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
		Name          string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
		Data          string             `xorm:"LONGTEXT NOT NULL"`
		Description   string             `xorm:"TEXT"`
		CreatedUnix   timeutil.TimeStamp `xorm:"created NOT NULL"`
		UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
	}
	return x.Sync(new(ActionEnvironment), new(Secret), new(ActionVariable))
}
