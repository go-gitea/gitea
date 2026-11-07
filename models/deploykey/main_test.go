// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package deploykey

import (
	"testing"

	"gitea.dev/models/unittest"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{FixtureFiles: []string{}}) // the tests insert what they assert on
}
