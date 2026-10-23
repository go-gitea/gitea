// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"codenest.dev/models/unittest"

	_ "codenest.dev/models" // register table model
	_ "codenest.dev/models/actions"
	_ "codenest.dev/models/activities"
	_ "codenest.dev/models/perm/access" // register table model
	_ "codenest.dev/models/repo"        // register table model
	_ "codenest.dev/models/user"        // register table model
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
