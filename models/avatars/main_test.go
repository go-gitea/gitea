// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatars_test

import (
	"testing"

	"codenest.dev/models/unittest"

	_ "codenest.dev/models"
	_ "codenest.dev/models/activities"
	_ "codenest.dev/models/perm/access"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
