// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"xorm.io/xorm"
)

// MigrateCommitIDOfPullRequestCodeReviewComment this will be almost right before comment on the special commit of the pull request
func MigrateCommitIDOfPullRequestCodeReviewComment(x *xorm.Engine) error {
	_, err := x.Exec("UPDATE comment SET commit_sha = (select merge_base from pull_request WHERE issue_id = comment.issue_id) WHERE line < 0")
	return err
}
