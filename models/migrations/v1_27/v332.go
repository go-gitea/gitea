// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"xorm.io/xorm"
)

// AddCommitSHAToIssue adds a commit_sha column to the issue table so that
// inline comments on individual commits can be modelled as a synthetic
// "commit-comment" Issue row per (repo, sha). Comments hang off that Issue
// the same way they do for ordinary issues / pull requests, which means
// existing infrastructure (CreateComment, UpdateCommentAttachments,
// DeleteOrphanedAttachments, the comment timeline UI, notifications, …)
// continues to work without any further schema churn.
//
// The new column is indexed so that GetIssueByRepoIDAndCommitSHA stays cheap
// regardless of how many issues a repository accumulates.
//
// Pre-existing issues (issue type "issue" / pull request) leave the column
// blank, which is fine — it just means they aren't commit-comment carriers.
func AddCommitSHAToIssue(x *xorm.Engine) error {
	type Issue struct {
		ID        int64  `xorm:"pk autoincr"`
		CommitSHA string `xorm:"INDEX VARCHAR(64) NOT NULL DEFAULT ''"`
	}

	return x.Sync(new(Issue))
}
