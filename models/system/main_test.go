// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system_test

import (
	"testing"

	"gitea.dev/models/unittest"

	_ "gitea.dev/models" // register models
	_ "gitea.dev/models/actions"
	_ "gitea.dev/models/activities"
	_ "gitea.dev/models/system" // register models of system
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
