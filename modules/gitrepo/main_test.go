// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

const (
	testReposDir = "../git/tests/repos/"
)

func TestMain(m *testing.M) {
	// since all test repository is defined by testReposDir, we need to set it here
	// So that all functions could work properly because Repository will only contain
	// Relative Path to RepoRootPath
	setting.RepoRootPath, _ = filepath.Abs(testReposDir)
	// TODO: run command requires a home directory, we could set it to a temp dir
	gitHomePath, err := os.MkdirTemp(os.TempDir(), "git-home")
	if err != nil {
		panic(fmt.Errorf("unable to create temp dir: %w", err))
	}
	defer util.RemoveAll(gitHomePath)
	setting.Git.HomePath = gitHomePath

	exitStatus := m.Run()
	os.Exit(exitStatus)
}
