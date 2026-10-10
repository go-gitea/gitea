// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm"
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
	if err := x.Sync(new(ActionEnvironment)); err != nil {
		return err
	}

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
	if err := addEnvironmentIDToUniqueName(x, new(Secret)); err != nil {
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
	if err := addEnvironmentIDToUniqueName(x, new(ActionVariable)); err != nil {
		return err
	}

	// a partial struct declares no indices, and a plain Sync would drop every index it does not find
	type ActionRunJob struct {
		EnvironmentName string `xorm:"TEXT"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreConstrains: true, IgnoreDropIndices: true}, new(ActionRunJob))
	return err
}

// addEnvironmentIDToUniqueName adds the column first, as RecreateTable copies the columns of the existing table,
// then rebuilds the table so that its unique name index covers environment_id.
func addEnvironmentIDToUniqueName(x base.EngineMigration, table any) error {
	if _, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreConstrains: true, IgnoreIndices: true, IgnoreDropIndices: true}, table); err != nil {
		return err
	}
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := base.RecreateTable(sess, table); err != nil {
		return err
	}
	return sess.Commit()
}
