// Copyright 2022 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/models/db"
)

const ForeignTypeIssue = "issue"

// ForeignReference represents external references
type ForeignReference struct {
	// RepoID is the first column in all indices. now we only need 2 indices: (repo, local) and (repo, foreign, type)
	RepoID    int64  `xorm:"UNIQUE(repo_foreign_type) INDEX(repo_local)" `
	LocalID   int64  `xorm:"INDEX(repo_local)"` // the resource key inside Gitea, it can be IssueIndex, or some model ID.
	ForeignID string `xorm:"INDEX UNIQUE(repo_foreign_type)"`
	Type      string `xorm:"VARCHAR(8) INDEX UNIQUE(repo_foreign_type)"`
}

func init() {
	db.RegisterModel(new(ForeignReference))
}
