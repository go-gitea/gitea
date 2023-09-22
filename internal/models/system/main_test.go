// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system_test

import (
	"testing"

	"code.gitea.io/gitea/internal/models/unittest"

	_ "code.gitea.io/gitea/internal/models" // register models
	_ "code.gitea.io/gitea/internal/models/actions"
	_ "code.gitea.io/gitea/internal/models/activities"
	_ "code.gitea.io/gitea/internal/models/system" // register models of system
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{})
}
