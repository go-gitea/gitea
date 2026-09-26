// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"

	_ "gitea.dev/models"
	_ "gitea.dev/models/actions"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		SetUp: func() error {
			setting.Webhook.AllowedHostList = "loopback"
			setting.LoadQueueSettings()
			return Init()
		},
	})
}
