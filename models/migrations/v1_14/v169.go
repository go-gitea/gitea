// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import "gitea.dev/models/db"

func CommentTypeDeleteBranchUseOldRef(x db.EngineMigration) error {
	_, err := x.Exec("UPDATE comment SET old_ref = commit_sha, commit_sha = '' WHERE type = 11")
	return err
}
