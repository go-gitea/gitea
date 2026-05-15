// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package usergroup_test

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	_ "code.gitea.io/gitea/models"
	_ "code.gitea.io/gitea/models/actions"
	_ "code.gitea.io/gitea/models/activities"
	_ "code.gitea.io/gitea/models/organization"
	_ "code.gitea.io/gitea/models/repo"
	_ "code.gitea.io/gitea/models/user"
	_ "code.gitea.io/gitea/models/usergroup"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
