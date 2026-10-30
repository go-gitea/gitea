// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	"gitea.dev/models/unittest"
	notify_service "gitea.dev/services/notify"

	_ "gitea.dev/models"
	_ "gitea.dev/models/actions"
)

var labelChanges = &labelChangeRecorder{}

func TestMain(m *testing.M) {
	notify_service.RegisterNotifier(labelChanges)
	unittest.MainTest(m)
}
