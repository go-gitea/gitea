// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_17 //nolint

import (
	"fmt"

	"xorm.io/xorm"
)

func CreateForeignReferenceTable(x *xorm.Engine) error {
	type ForeignReference struct {
		// RepoID is the first column in all indices. now we only need 2 indices: (repo, local) and (repo, foreign, type)
		RepoID       int64  `xorm:"UNIQUE(repo_foreign_type) INDEX(repo_local)" `
		LocalIndex   int64  `xorm:"INDEX(repo_local)"` // the resource key inside Gitea, it can be IssueIndex, or some model ID.
		ForeignIndex string `xorm:"INDEX UNIQUE(repo_foreign_type)"`
		Type         string `xorm:"VARCHAR(16) INDEX UNIQUE(repo_foreign_type)"`
	}

	if err := x.Sync2(new(ForeignReference)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
