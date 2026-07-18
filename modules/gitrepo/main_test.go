// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"path/filepath"
	"testing"

	"gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
)

func mockRepository(repoPath string) repo.StorageRepo {
	if !filepath.IsAbs(repoPath) {
		// resolve repository path relative to the unit test fixture directory
		repoPath = filepath.Join(setting.GetGiteaTestSourceRoot(), "modules/git/tests/repos", repoPath)
	}
	return repo.StorageRepo(repoPath)
}

func TestMain(m *testing.M) {
	setting.SetupGiteaTestEnv()
	git.RunGitTests(m)
}
