// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatars_test

import (
	"testing"

	_ "code.gitea.io/gitea/models"
	_ "code.gitea.io/gitea/models/activities"
	_ "code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/models/unittest"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
