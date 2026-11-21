// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"
)

// AddDisclosureTables adds the tables for repository security disclosures.
// The structs mirror models/disclosure and must stay in sync with them.
func AddDisclosureTables(_ context.Context, x base.EngineMigration) error {
	type Disclosure struct {
		ID            int64              `xorm:"pk autoincr"`
		RepoID        int64              `xorm:"INDEX UNIQUE(repo_num)"`
		Num           int64              `xorm:"UNIQUE(repo_num)"`
		AuthorID      int64              `xorm:"INDEX"`
		Title         string             `xorm:"VARCHAR(255) NOT NULL"`
		Summary       string             `xorm:"TEXT"`
		Content       string             `xorm:"LONGTEXT"`
		Severity      string             `xorm:"VARCHAR(16) INDEX NOT NULL DEFAULT 'none'"`
		CVSSVector    string             `xorm:"VARCHAR(128)"`
		CweIds        string             `xorm:"TEXT"`
		CveID         string             `xorm:"VARCHAR(64)"`
		References    string             `xorm:"TEXT"`
		Status        string             `xorm:"VARCHAR(16) INDEX NOT NULL DEFAULT 'draft'"`
		PublishedAt   timeutil.TimeStamp `xorm:"INDEX"`
		PublishedByID int64              `xorm:"INDEX"`
		CreatedUnix   timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
	}
	type DisclosureVersion struct {
		ID           int64  `xorm:"pk autoincr"`
		DisclosureID int64  `xorm:"INDEX UNIQUE(dv)"`
		Kind         string `xorm:"VARCHAR(16) UNIQUE(dv)"`
		Version      string `xorm:"VARCHAR(255) UNIQUE(dv)"`
	}
	return x.Sync(new(Disclosure), new(DisclosureVersion))
}
