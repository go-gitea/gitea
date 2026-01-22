// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

// getRepoWriteLockKey returns the global lock key for write operations on the repository.
// Parallel write operations on the same git repository should be avoided to prevent data corruption.
func getRepoWriteLockKey(repoStoragePath string) string {
	return "repo-write:" + repoStoragePath
}
