// Copyright 2022 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	"code.gitea.io/gitea/models/migrations/base"
	"xorm.io/xorm"
)

func AddPrimaryKeyToForeignReference(x *xorm.Engine) error {
	// ForeignReference represents external references
	type ForeignReference struct {
		ID int64 `xorm:"pk autoincr"`

		// RepoID is the first column in all indices. now we only need 2 indices: (repo, local) and (repo, foreign, type)
		RepoID       int64  `xorm:"UNIQUE(repo_foreign_type) INDEX(repo_local)" `
		LocalIndex   int64  `xorm:"INDEX(repo_local)"` // the resource key inside Gitea, it can be IssueIndex, or some model ID.
		ForeignIndex string `xorm:"INDEX UNIQUE(repo_foreign_type)"`
		Type         string `xorm:"VARCHAR(16) INDEX UNIQUE(repo_foreign_type)"`
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	base.RecreateTable(sess, &ForeignReference{})

	return sess.Commit()
}
