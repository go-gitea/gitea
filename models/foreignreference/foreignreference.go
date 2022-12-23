// Copyright 2022 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package foreignreference

import (
	"code.gitea.io/gitea/models/db"
)

// Type* are valid values for the Type field of ForeignReference
const (
	TypeIssue         = "issue"
	TypePullRequest   = "pull_request"
	TypeComment       = "comment"
	TypeReview        = "review"
	TypeReviewComment = "review_comment"
	TypeRelease       = "release"
)

// ForeignReference represents external references
type ForeignReference struct {
	// RepoID is the first column in all indices. now we only need 2 indices: (repo, local) and (repo, foreign, type)
	RepoID       int64  `xorm:"UNIQUE(repo_foreign_type) INDEX(repo_local)" `
	LocalIndex   int64  `xorm:"INDEX(repo_local)"` // the resource key inside Gitea, it can be IssueIndex, or some model ID.
	ForeignIndex string `xorm:"INDEX UNIQUE(repo_foreign_type)"`
	Type         string `xorm:"VARCHAR(16) INDEX UNIQUE(repo_foreign_type)"`
}

func init() {
	db.RegisterModel(new(ForeignReference))
}
