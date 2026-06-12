// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"
)

type RepoContributorDaily struct {
	ID           int64  `xorm:"pk autoincr"`
	RepoID       int64  `xorm:"UNIQUE(repo_user_day) INDEX NOT NULL"`
	DayStart     int64  `xorm:"UNIQUE(repo_user_day) INDEX NOT NULL"`
	UserID       int64  `xorm:"UNIQUE(repo_user_day) INDEX NOT NULL DEFAULT 0"`
	Email        string `xorm:"UNIQUE(repo_user_day) INDEX VARCHAR(255) NOT NULL DEFAULT ''"`
	AuthorName   string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	Additions    int64  `xorm:"NOT NULL DEFAULT 0"`
	Deletions    int64  `xorm:"NOT NULL DEFAULT 0"`
	Commits      int64  `xorm:"NOT NULL DEFAULT 0"`
	ChangedFiles int64  `xorm:"NOT NULL DEFAULT 0"`
	UpdatedUnix  int64  `xorm:"INDEX updated"`
}

type RepoContributorMeta struct {
	RepoID                int64  `xorm:"pk"`
	LastProcessedCommitID string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	Dirty                 bool   `xorm:"NOT NULL DEFAULT false"`
	UpdatedUnix           int64  `xorm:"INDEX updated"`
}

// AddRepoContributorDailyAndMeta creates tables for contributor daily stats.
func AddRepoContributorDailyAndMeta(x db.EngineMigration) error {
	return x.Sync(new(RepoContributorDaily), new(RepoContributorMeta))
}
