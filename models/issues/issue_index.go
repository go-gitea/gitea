// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import "code.gitea.io/gitea/models/db"

// RecalculateIssueIndexForRepo create issue_index for repo if not exist and
// update it based on highest index of existing issues assigned to a repo
func RecalculateIssueIndexForRepo(repoID int64) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := db.UpsertResourceIndex(ctx, "issue_index", repoID); err != nil {
		return err
	}

	var max int64
	if _, err := db.GetEngine(ctx).Select(" MAX(`index`)").Table("issue").Where("repo_id=?", repoID).Get(&max); err != nil {
		return err
	}

	if _, err := db.GetEngine(ctx).Exec("UPDATE `issue_index` SET max_index=? WHERE group_id=?", max, repoID); err != nil {
		return err
	}

	return committer.Commit()
}
