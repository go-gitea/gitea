// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"codenest.dev/models/unittest"
	notify_service "codenest.dev/services/notify"

	_ "codenest.dev/models/actions"
	_ "codenest.dev/models/activities"
)

var labelChanges = &labelChangeRecorder{}

func TestMain(m *testing.M) {
	notify_service.RegisterNotifier(labelChanges)
	unittest.MainTest(m)
}
