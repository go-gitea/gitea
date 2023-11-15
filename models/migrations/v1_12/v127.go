// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12 //nolint

import (
	"fmt"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddLanguageStats(x *xorm.Engine) error {
	// LanguageStat see models/repo_language_stats.go
	type LanguageStat struct {
		ID          int64 `xorm:"pk autoincr"`
		RepoID      int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
		CommitID    string
		IsPrimary   bool
		Language    string             `xorm:"VARCHAR(30) UNIQUE(s) INDEX NOT NULL"`
		Percentage  float32            `xorm:"NUMERIC(5,2) NOT NULL DEFAULT 0"`
		Color       string             `xorm:"-"`
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX CREATED"`
	}

	type RepoIndexerType int

	// RepoIndexerStatus see models/repo_stats_indexer.go
	type RepoIndexerStatus struct {
		ID          int64           `xorm:"pk autoincr"`
		RepoID      int64           `xorm:"INDEX(s)"`
		CommitSha   string          `xorm:"VARCHAR(40)"`
		IndexerType RepoIndexerType `xorm:"INDEX(s) NOT NULL DEFAULT 0"`
	}

	if err := x.Sync(new(LanguageStat)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	if err := x.Sync(new(RepoIndexerStatus)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
