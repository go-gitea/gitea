// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

var lockFiles = []string{
	"config.lock",
	"objects/info/commit-graphs/commit-graph-chain.lock",
}

// CleanupRepo cleans up the repository by removing unnecessary lock files.
func CleanupRepo(ctx context.Context, repo Repository) error {
	return CleanFixedFileLocks(ctx, repo, time.Now())
}

// CleanFixedFileLocks removes lock files that haven't been modified since the last update.
func CleanFixedFileLocks(ctx context.Context, repo Repository, lastUpdated time.Time) error {
	for _, lockFile := range lockFiles {
		p := filepath.Join(repoPath(repo), lockFile)
		fInfo, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}

		if fInfo.ModTime().Before(lastUpdated) {
			if err := os.Remove(p); err != nil {
				return err
			}
		}
	}
	return nil
}
