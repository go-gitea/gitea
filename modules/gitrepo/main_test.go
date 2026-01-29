// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
)

func TestMain(m *testing.M) {
	// resolve repository path relative to the test directory
	testRootDir := setting.SetupGiteaTestEnv()
	repoPath = func(repo Repository) string {
		if filepath.IsAbs(repo.RelativePath()) {
			return repo.RelativePath() // for testing purpose only
		}
		return filepath.Join(testRootDir, "modules/git/tests/repos", repo.RelativePath())
	}
	git.RunGitTests(m)
}
