// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/unittest"

	_ "code.gitea.io/gitea/models"             // register table model
	_ "code.gitea.io/gitea/models/perm/access" // register table model
	_ "code.gitea.io/gitea/models/repo"        // register table model
	_ "code.gitea.io/gitea/models/user"        // register table model
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: filepath.Join("..", ".."),
	})
}
