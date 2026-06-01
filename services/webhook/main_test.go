// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/hostmatcher"
	"gitea.dev/modules/setting"

	_ "gitea.dev/models"
	_ "gitea.dev/models/actions"
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
