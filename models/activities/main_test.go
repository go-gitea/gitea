// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities_test

import (
	"testing"

	_ "code.gitea.io/gitea/models"
	_ "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/unittest"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
