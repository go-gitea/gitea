// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	"codenest.dev/models/unittest"
	notify_service "codenest.dev/services/notify"

	_ "codenest.dev/models"
	_ "codenest.dev/models/actions"
)

var labelChanges = &labelChangeRecorder{}

func TestMain(m *testing.M) {
	notify_service.RegisterNotifier(labelChanges)
	unittest.MainTest(m)
}
