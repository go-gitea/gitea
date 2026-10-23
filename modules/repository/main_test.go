// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"codenest.dev/models/unittest"

	_ "codenest.dev/models"
	_ "codenest.dev/models/actions"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
