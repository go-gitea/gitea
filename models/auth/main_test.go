// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	_ "code.gitea.io/gitea/models"
	_ "code.gitea.io/gitea/models/actions"
	_ "code.gitea.io/gitea/models/activities"
	_ "code.gitea.io/gitea/models/auth"
	_ "code.gitea.io/gitea/models/perm/access"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
