// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_15

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/migrationtest"
)

func TestMain(m *testing.M) {
	migrationtest.MainTest(m)
}
