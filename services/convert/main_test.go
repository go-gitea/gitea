// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"testing"

	"gitea.dev/models/unittest"

	_ "gitea.dev/models/actions"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
