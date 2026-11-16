// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"gitea.dev/models/unittest"
	notify_service "gitea.dev/services/notify"

	_ "gitea.dev/models/actions"
	_ "gitea.dev/models/activities"
)

var labelChanges = &labelChangeRecorder{}

func TestMain(m *testing.M) {
	notify_service.RegisterNotifier(labelChanges)
	unittest.MainTest(m)
}
