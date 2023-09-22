// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"code.gitea.io/gitea/internal/models/unittest"

	_ "code.gitea.io/gitea/internal/models" // register table model
	_ "code.gitea.io/gitea/internal/models/actions"
	_ "code.gitea.io/gitea/internal/models/activities"
	_ "code.gitea.io/gitea/internal/models/perm/access" // register table model
	_ "code.gitea.io/gitea/internal/models/repo"        // register table model
	_ "code.gitea.io/gitea/internal/models/user"        // register table model
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{})
}
