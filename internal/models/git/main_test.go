// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git_test

import (
	"testing"

	"code.gitea.io/gitea/internal/models/unittest"

	_ "code.gitea.io/gitea/internal/models"
	_ "code.gitea.io/gitea/internal/models/actions"
	_ "code.gitea.io/gitea/internal/models/activities"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{})
}
