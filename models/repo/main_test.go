// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"gitea.dev/models/unittest"

	_ "gitea.dev/models" // register table model
	_ "gitea.dev/models/actions"
	_ "gitea.dev/models/activities"
	_ "gitea.dev/models/perm/access" // register table model
	_ "gitea.dev/models/repo"        // register table model
	_ "gitea.dev/models/user"        // register table model
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
