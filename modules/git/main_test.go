// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"gitea.dev/modules/git/gitrepo"
)

const testReposDir = "tests/repos/"

func mockRepository(repoPath string) RepositoryFacade {
	if !filepath.IsAbs(repoPath) {
		// resolve repository path relative to the unit test fixture directory
		repoPath, _ = filepath.Abs(filepath.Join(testReposDir, repoPath))
	}
	return gitrepo.RepositoryUnmanaged(repoPath)
}

func TestMain(m *testing.M) {
	RunGitTests(m)
}
