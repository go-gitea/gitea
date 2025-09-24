// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/setting"
)

const (
	testReposDir = "../git/tests/repos/"
)

func TestMain(m *testing.M) {
	setting.RepoRootPath, _ = filepath.Abs(testReposDir)
	setting.Git.HomePath = filepath.Join(setting.RepoRootPath, ".home")

	exitStatus := m.Run()
	os.Exit(exitStatus)
}
