// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatars_test

import (
	"testing"

	"gitea.dev/models/unittest"

	_ "gitea.dev/models"
	_ "gitea.dev/models/activities"
	_ "gitea.dev/models/perm/access"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
