// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/tempdir"
	"code.gitea.io/gitea/modules/test"
)

func TestMain(m *testing.M) {
	gitHomePath, cleanup, err := tempdir.OsTempDir("gitea-test").MkdirTempRandom("git-home")
	if err != nil {
		log.Fatal("Unable to create temp dir: %v", err)
	}
	defer cleanup()

	// resolve repository path relative to the test directory
	testRootDir := test.SetupGiteaRoot()
	repoPath = func(repo Repository) string {
		return filepath.Join(testRootDir, "/modules/git/tests/repos", repo.RelativePath())
	}

	setting.Git.HomePath = gitHomePath
	os.Exit(m.Run())
}
