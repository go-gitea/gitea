// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	"code.gitea.io/gitea/internal/models/unittest"
	"code.gitea.io/gitea/internal/modules/hostmatcher"
	"code.gitea.io/gitea/internal/modules/setting"

	_ "code.gitea.io/gitea/internal/models"
	_ "code.gitea.io/gitea/internal/models/actions"
)

func TestMain(m *testing.M) {
	// for tests, allow only loopback IPs
	setting.Webhook.AllowedHostList = hostmatcher.MatchBuiltinLoopback
	unittest.MainTest(m, &unittest.TestOptions{
		SetUp: func() error {
			setting.LoadQueueSettings()
			return Init()
		},
	})
}
