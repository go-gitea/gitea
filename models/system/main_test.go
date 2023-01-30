// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system_test

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/unittest"

	_ "code.gitea.io/gitea/models"        // register models
	_ "code.gitea.io/gitea/models/system" // register models of system
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: filepath.Join("..", ".."),
	})
}
