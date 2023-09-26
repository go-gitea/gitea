// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	_ "code.gitea.io/gitea/models/actions"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{})
}
