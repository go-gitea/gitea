// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"gitea.dev/models/unittest"

	_ "gitea.dev/models/actions"
	_ "gitea.dev/models/activities"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
