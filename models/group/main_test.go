// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package group_test

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	_ "code.gitea.io/gitea/models/group"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
