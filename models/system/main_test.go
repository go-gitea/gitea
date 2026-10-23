// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system_test

import (
	"testing"

	"codenest.dev/models/unittest"

	_ "codenest.dev/models" // register models
	_ "codenest.dev/models/actions"
	_ "codenest.dev/models/activities"
	_ "codenest.dev/models/system" // register models of system
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
