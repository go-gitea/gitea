// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	webhook_service "code.gitea.io/gitea/services/webhook"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		SetUp: webhook_service.Init,
	})
}
