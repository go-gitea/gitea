// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	_ "code.gitea.io/gitea/models/user"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
