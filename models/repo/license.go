// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"code.gitea.io/gitea/models/db"
)

// UpdateLanguageStats updates the license statistics for repository
func UpdateLicenseStats(repo *Repository, commitID string, licenses []string) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = UpdateLicense(ctx, repo.ID, licenses); err != nil {
		return err
	}

	// Update indexer status
	if err = UpdateIndexerStatus(ctx, repo, RepoIndexerTypeStats, commitID); err != nil {
		return err
	}

	return committer.Commit()
}

// UpdateRepoSize updates the repository size, calculating it using getDirectorySize
func UpdateLicense(ctx context.Context, repoID int64, licenses []string) error {
	_, err := db.GetEngine(ctx).ID(repoID).Cols("licenses").NoAutoTime().Update(&Repository{
		Licenses: licenses,
	})
	return err
}
