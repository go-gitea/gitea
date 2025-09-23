// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
)

const (
	testReposDir = "../git/tests/repos/"
)

func TestMain(m *testing.M) {
	originalRepoRootPath := setting.RepoRootPath
	defer func() {
		setting.RepoRootPath = originalRepoRootPath
	}()
	setting.RepoRootPath, _ = filepath.Abs(testReposDir)

	originalHomePath := setting.Git.HomePath
	defer func() {
		setting.Git.HomePath = originalHomePath
	}()
	setting.Git.HomePath = filepath.Join(setting.RepoRootPath, ".home")

	originalGitPath := setting.Git.Path
	defer func() {
		setting.Git.Path = originalGitPath
	}()
	setting.Git.Path = "git"

	if err := git.InitSimple(); err != nil {
		panic(err)
	}

	exitStatus := m.Run()
	os.Exit(exitStatus)
}
