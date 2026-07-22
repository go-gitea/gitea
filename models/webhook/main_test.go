// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	"gitea.dev/models/unittest"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		FixtureFiles: []string{
			"webhook.yml",
			"hook_task.yml",
		},
		SetUp: prepareWebhookTestData,
	})
}
