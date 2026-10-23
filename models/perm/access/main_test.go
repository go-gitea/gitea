// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package access_test

import (
	"testing"

	"codenest.dev/models/unittest"

	_ "codenest.dev/models"
	_ "codenest.dev/models/actions"
	_ "codenest.dev/models/activities"
	_ "codenest.dev/models/repo"
	_ "codenest.dev/models/user"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
