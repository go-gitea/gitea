// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "code.gitea.io/gitea/models"

// UpdateGithubMigrations will update posterid on issues/comments/prs when github user
// login or when migrating
func UpdateGithubMigrations(repoID, githubUserID, userID int64) error {
	if err := models.UpdateIssuesMigrations(repoID, githubUserID, userID); err != nil {
		return err
	}

	return models.UpdateCommentsMigrations(repoID, githubUserID, userID)
}
