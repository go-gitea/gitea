// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddRepoContributorTables(_ context.Context, x base.EngineMigration) error {
	type RepoContributor struct {
		ID        int64  `xorm:"pk autoincr"`
		RepoID    int64  `xorm:"UNIQUE(s) NOT NULL"`
		UserID    int64  `xorm:"UNIQUE(s) NOT NULL DEFAULT 0"`
		Email     string `xorm:"UNIQUE(s) VARCHAR(255) NOT NULL DEFAULT ''"`
		Name      string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
		Commits   int64  `xorm:"NOT NULL DEFAULT 0"`
		Additions int64  `xorm:"NOT NULL DEFAULT 0"`
		Deletions int64  `xorm:"NOT NULL DEFAULT 0"`
	}
	type RepoContributorWeek struct {
		ID        int64  `xorm:"pk autoincr"`
		RepoID    int64  `xorm:"UNIQUE(s) NOT NULL"`
		UserID    int64  `xorm:"UNIQUE(s) NOT NULL DEFAULT 0"`
		Email     string `xorm:"UNIQUE(s) VARCHAR(255) NOT NULL DEFAULT ''"`
		Week      int64  `xorm:"UNIQUE(s) NOT NULL"`
		Commits   int64  `xorm:"NOT NULL DEFAULT 0"`
		Additions int64  `xorm:"NOT NULL DEFAULT 0"`
		Deletions int64  `xorm:"NOT NULL DEFAULT 0"`
	}
	return x.Sync(new(RepoContributor), new(RepoContributorWeek))
}
