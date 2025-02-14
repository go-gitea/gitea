// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	_ "code.gitea.io/gitea/models/actions"
	_ "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/unittest"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
