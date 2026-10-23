// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	"codenest.dev/models/unittest"
	"codenest.dev/modules/setting"

	_ "codenest.dev/models"
	_ "codenest.dev/models/actions"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		SetUp: func() error {
			setting.Webhook.AllowedHostList = "loopback:*"
			setting.LoadQueueSettings()
			return Init()
		},
	})
}
