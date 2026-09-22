// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/egress/policy"
	"gitea.dev/modules/setting"

	_ "gitea.dev/models"
	_ "gitea.dev/models/actions"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		SetUp: func() error {
			// for tests, allow only loopback IPs. This must run after the test config is loaded
			// (which resets the allow list) and before Init() builds the delivery client's policy.
			setting.Webhook.AllowedHostList = policy.MatchBuiltinLoopback
			setting.LoadQueueSettings()
			return Init()
		},
	})
}
