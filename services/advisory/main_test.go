// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"testing"

	"gitea.dev/models/unittest"

	_ "gitea.dev/models"
	_ "gitea.dev/models/actions"
	_ "gitea.dev/models/activities"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
