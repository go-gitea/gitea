// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	_ "code.gitea.io/gitea/models"
	_ "code.gitea.io/gitea/models/actions"
	_ "code.gitea.io/gitea/models/activities"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
