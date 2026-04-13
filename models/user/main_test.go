// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user_test

import (
	"testing"

	_ "code.gitea.io/gitea/models"
	_ "code.gitea.io/gitea/models/actions"
	_ "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/unittest"
	_ "code.gitea.io/gitea/models/user"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
